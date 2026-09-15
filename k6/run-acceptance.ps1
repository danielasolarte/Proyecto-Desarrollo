$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force "k6/results" | Out-Null
k6 run `
  -e PROFILE=acceptance `
  -e STUDENT_POOL=100 `
  --summary-export "k6/results/acceptance-summary.json" `
  "k6/full-project.js"
