# ── VPC ────────────────────────────────────────────────────────────
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${var.cluster_name}-vpc"
  cidr = var.vpc_cidr

  azs             = var.availability_zones
  private_subnets = var.private_subnet_cidrs
  public_subnets  = var.public_subnet_cidrs

  enable_nat_gateway     = var.environment != "dev" ? true : false
  single_nat_gateway     = var.environment == "dev" ? true : false
  one_nat_gateway_per_az = false
  enable_vpn_gateway     = false

  enable_dns_hostnames = true
  enable_dns_support   = true

  public_subnet_tags = {
    "kubernetes.io/role/elb" = "1"
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = "1"
  }

  tags = var.tags
}

# ── Security Group for EKS ────────────────────────────────────────
resource "aws_security_group" "cluster" {
  name        = "${var.cluster_name}-cluster-sg"
  description = "EKS cluster security group"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "Allow all internal traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [var.vpc_cidr]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = var.tags
}

# ── EKS Cluster ───────────────────────────────────────────────────
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version

  cluster_endpoint_public_access = var.environment == "dev" ? true : false

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  # Security groups
  cluster_security_group_id = aws_security_group.cluster.id

  # EKS Managed Node Group
  eks_managed_node_groups = {
    main = {
      desired_size = var.node_group_desired_size
      min_size     = var.node_group_min_size
      max_size     = var.node_group_max_size

      instance_types = var.node_group_instance_types

      block_device_mappings = {
        xvda = {
          device_name = "/dev/xvda"
          ebs = {
            volume_size           = 50
            volume_type           = "gp3"
            encrypted             = true
            delete_on_termination = true
          }
        }
      }

      tags = var.tags
    }
  }

  # Cluster add-ons
  cluster_addons = {
    coredns = {
      most_recent = true
    }
    kube-proxy = {
      most_recent = true
    }
    vpc-cni = {
      most_recent = true
    }
  }

  tags = var.tags
}

# ── Node Group Scaling Schedule (dev: scale to 0 at night) ────────
resource "aws_autoscaling_schedule" "night_scale_in" {
  count = var.environment == "dev" ? 1 : 0

  scheduled_action_name  = "kubewatcher-night-scale-in"
  min_size               = 0
  max_size               = 0
  desired_capacity       = 0
  recurrence             = "0 20 * * *"
  autoscaling_group_name = module.eks.eks_managed_node_groups["main"]

  time_zone = "America/Los_Angeles"
}

resource "aws_autoscaling_schedule" "morning_scale_out" {
  count = var.environment == "dev" ? 1 : 0

  scheduled_action_name  = "kubewatcher-morning-scale-out"
  min_size               = var.node_group_min_size
  max_size               = var.node_group_max_size
  desired_capacity       = var.node_group_desired_size
  recurrence             = "0 7 * * 1-5"
  autoscaling_group_name = module.eks.eks_managed_node_groups["main"]

  time_zone = "America/Los_Angeles"
}
