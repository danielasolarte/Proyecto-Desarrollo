$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force "k6/results" | Out-Null

docker run --rm `
  -v "${PWD}:/project" `
  -w /project `
  grafana/k6 run `
  -e PROFILE=smoke `
  -e BASE_URL=http://host.docker.internal:8080/api/v1 `
  --summary-export /project/k6/results/smoke-summary.json `
  k6/full-project.js