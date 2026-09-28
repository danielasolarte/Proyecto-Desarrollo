$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force "k6/results" | Out-Null

if (-not $env:STUDENT_EMAIL) {
    throw "Falta STUDENT_EMAIL"
}

if (-not $env:STUDENT_PASSWORD) {
    throw "Falta STUDENT_PASSWORD"
}

if (-not $env:QUIZ_ID) {
    throw "Falta QUIZ_ID"
}

if (-not $env:QUESTION_ID) {
    throw "Falta QUESTION_ID"
}

if (-not $env:CORRECT_OPTION_ID) {
    throw "Falta CORRECT_OPTION_ID"
}

docker run --rm `
  -v "${PWD}:/project" `
  -w /project `
  grafana/k6 run `
  -e BASE_URL=http://host.docker.internal:8080/api/v1 `
  -e STUDENT_EMAIL="$env:STUDENT_EMAIL" `
  -e STUDENT_PASSWORD="$env:STUDENT_PASSWORD" `
  -e QUIZ_ID="$env:QUIZ_ID" `
  -e QUESTION_ID="$env:QUESTION_ID" `
  -e CORRECT_OPTION_ID="$env:CORRECT_OPTION_ID" `
  --summary-export /project/k6/results/idempotency-summary.json `
  k6/idempotency.js