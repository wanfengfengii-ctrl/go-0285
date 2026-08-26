# Buildx bake configuration producing linux/amd64 and linux/arm64 images.
variable "TAG" {
  default = "precast-wall-grout-support-release"
}

group "default" {
  targets = ["amd64", "arm64"]
}

target "base" {
  platforms = ["linux/amd64", "linux/arm64"]
  tags       = ["${TAG}:latest"]
}

target "amd64" {
  inherits = ["base"]
  platforms = ["linux/amd64"]
  tags       = ["${TAG}:amd64"]
}

target "arm64" {
  inherits = ["base"]
  platforms = ["linux/arm64"]
  tags       = ["${TAG}:arm64"]
}
