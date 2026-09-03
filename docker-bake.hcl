// docker-bake.hcl drives the five release images from one file. Tags are not
// stored here: the release workflow passes them per target with the `set`
// input, so this file never needs editing when versions change.
//
//   docker buildx bake                 # all five images, host arch, no push
//   docker buildx bake --push api      # one image, both platforms, pushed

group "default" {
  targets = ["api", "auth", "redirect", "worker", "frontend"]
}

target "api" {
  context    = "."
  dockerfile = "Dockerfile"
  target     = "api"
  platforms  = ["linux/amd64", "linux/arm64"]
}

target "auth" {
  context    = "."
  dockerfile = "Dockerfile"
  target     = "auth"
  platforms  = ["linux/amd64", "linux/arm64"]
}

target "redirect" {
  context    = "."
  dockerfile = "Dockerfile"
  target     = "redirector"
  platforms  = ["linux/amd64", "linux/arm64"]
}

target "worker" {
  context    = "."
  dockerfile = "Dockerfile"
  target     = "worker"
  platforms  = ["linux/amd64", "linux/arm64"]
}

target "frontend" {
  context    = "frontend"
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
}
