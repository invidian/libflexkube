resource "null_resource" "controller_ips" {
  count = var.controllers_count

  triggers = {
    name = format("controller%02d", count.index + 1)
    ip   = cidrhost(var.nodes_cidr, count.index + 2)
    cidr = cidrsubnet(var.pod_cidr, 8, count.index + 2)
  }
}

locals {
  controller_ips      = null_resource.controller_ips.*.triggers.ip
  first_controller_ip = local.controller_ips[0]
  controller_names    = null_resource.controller_ips.*.triggers.name
  controller_cidrs    = null_resource.controller_ips.*.triggers.cidr

  worker_ips = null_resource.workers.*.triggers.ip
  worker_cidrs = null_resource.workers.*.triggers.cidr
  worker_names = null_resource.workers.*.triggers.name
}

resource "null_resource" "workers" {
  count = var.workers_count

  triggers = {
    name = format("worker%02d", count.index + 1)
    ip   = cidrhost(var.nodes_cidr, count.index + 2 + var.controllers_count)
    cidr = cidrsubnet(var.pod_cidr, 8, count.index + 2 + var.controllers_count)
  }
}