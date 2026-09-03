// docker-bake.hcl drives the five release images from one file. Tags come
// from the VERSION and MINOR variables: the release workflow overrides them
// with same-named environment variables (bake's variable override), so this
// file never needs editing when versions change. Without them (a plain
// `docker buildx bake`), images are tagged :dev for local testing.
//
//   docker buildx bake                 # all five images, host arch, :dev tag
//   VERSION=0.1.0 MINOR=0.1 docker buildx bake --push

variable "REGISTRY" {
  default = "ghcr.io/barats"
}

variable "VERSION" {
  default = ""
}

variable "MINOR" {
  default = ""
}

function "tags" {
  params = [name]
  result = VERSION != "" ? [
    "${REGISTRY}/${name}:${VERSION}",
    "${REGISTRY}/${name}:${MINOR}",
    "${REGISTRY}/${name}:latest"
  ] : ["${REGISTRY}/${name}:dev"]
}

group "default" {
  targets = ["api", "auth", "redirect", "worker", "frontend"]
}

target "api" {
  context    = "."
  dockerfile = "Dockerfile"
  target     = "api"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = tags("shrl-io-api")
}

target "auth" {
  context    = "."
  dockerfile = "Dockerfile"
  target     = "auth"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = tags("shrl-io-auth")
}

target "redirect" {
  context    = "."
  dockerfile = "Dockerfile"
  target     = "redirector"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = tags("shrl-io-redirect")
}

target "worker" {
  context    = "."
  dockerfile = "Dockerfile"
  target     = "worker"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = tags("shrl-io-worker")
}

target "frontend" {
  context    = "frontend"
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags       = tags("shrl-io-frontend")
}
