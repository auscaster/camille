PGPASSWORD=password psql -h localhost -U postgres -d camille -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;"

curl -X POST 'http://localhost:8080/scan?wait=true' -H 'Content-Type: app
lication/json' -d '{"url":"https://facebook.com"}'