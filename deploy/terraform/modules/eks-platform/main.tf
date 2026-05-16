# ── VPC ────────────────────────────────────────────────────────────
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${var.cluster_name}-vpc"
  cidr = var.vpc_cidr

  azs             = var.availability_zones
  private_subnets = var.private_subnet_cidrs
  public_subnets  = var.public_subnet_cidrs

  enable_nat_gateway     = var.environment != "dev"
  single_nat_gateway     = var.environment == "dev"
  one_nat_gateway_per_az = false
  enable_vpn_gateway     = false

  enable_dns_hostnames = true
  enable_dns_support   = true

  public_subnet_tags  = { "kubernetes.io/role/elb" = "1" }
  private_subnet_tags = { "kubernetes.io/role/internal-elb" = "1" }

  tags = var.tags
}

# ── Cluster security group ────────────────────────────────────────
resource "aws_security_group" "cluster" {
  name        = "${var.cluster_name}-cluster-sg"
  description = "EKS cluster security group for ${var.cluster_name}"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "Internal traffic within the VPC"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [var.vpc_cidr]
  }

  egress {
    description = "Allow all egress"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = var.tags
}

# ── EKS
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version

  cluster_endpoint_public_access = var.cluster_endpoint_public_access

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  cluster_security_group_id = aws_security_group.cluster.id

  eks_managed_node_groups = {
    main = {
      desired_size = var.node_group_desired_size
      min_size     = var.node_group_min_size
      max_size     = var.node_group_max_size

      instance_types = var.node_group_instance_types

      use_custom_launch_template = false
      disk_size                  = 30

      block_device_mappings = {
        xvda = {
          device_name = "/dev/xvda"
          ebs = {
            volume_size           = 30
            volume_type           = "gp3"
            encrypted             = true
            delete_on_termination = true
          }
        }
      }

      tags = var.tags
    }
  }

  cluster_addons = {
    coredns    = { most_recent = true }
    kube-proxy = { most_recent = true }
    vpc-cni    = { most_recent = true }
  }

  tags = var.tags
}

# ── Spot instances (dev) ──────────────────────────────────────────
# EKS managed node groups support mixed instances with spot.
resource "aws_eks_node_group" "spot" {
  count = var.use_spot ? 1 : 0

  cluster_name    = var.cluster_name
  node_group_name = "${var.cluster_name}-spot"
  node_role_arn   = module.eks.eks_managed_node_groups["main"].node_group_arn
  subnet_ids      = module.vpc.private_subnets

  scaling_config {
    desired_size = var.node_group_min_size
    min_size     = var.node_group_min_size
    max_size     = var.node_group_max_size
  }

  capacity_type = "SPOT"

  instance_types = var.node_group_instance_types

  labels = {
    "kubewatcher.io/spot" = "true"
  }

  tags = var.tags
}

# ── Scheduled scaling (dev) ──────────────────────────────────────
resource "aws_autoscaling_schedule" "night_scale_in" {
  count = var.enable_night_scale_in ? 1 : 0

  scheduled_action_name  = "${var.cluster_name}-night-scale-in"
  min_size               = 0
  max_size               = 0
  desired_capacity       = 0
  recurrence             = var.scale_in_cron
  autoscaling_group_name = module.eks.eks_managed_node_groups["main"].node_group_autoscaling_group_names[0]

  time_zone = var.scaling_time_zone
}

resource "aws_autoscaling_schedule" "morning_scale_out" {
  count = var.enable_night_scale_in ? 1 : 0

  scheduled_action_name  = "${var.cluster_name}-morning-scale-out"
  min_size               = var.node_group_min_size
  max_size               = var.node_group_max_size
  desired_capacity       = var.node_group_desired_size
  recurrence             = var.scale_out_cron
  autoscaling_group_name = module.eks.eks_managed_node_groups["main"].node_group_autoscaling_group_names[0]

  time_zone = var.scaling_time_zone
}
