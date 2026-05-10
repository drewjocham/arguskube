# Scheduled scaling for non-prod clusters.
#
# Pattern: spin nodes down at night and back up on weekday mornings so
# the cluster pays for compute only while engineers are awake. Off by
# default; set enable_night_scale_in = true on dev environments.

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
