#!/usr/bin/env bash
# Se ejecuta en el Web Server. Lee DATABASE_URL del .env protegido (sin
# imprimirlo) y devuelve una linea por media_asset separada por '|':
# id|kind|original_key|size|sha256|hls_manifest_key|converted_pdf_key|processing_status
set -euo pipefail
cd /opt/mooc/deploy
sudo docker run --rm -i --network host --env-file .env postgres:16 \
  sh -c 'psql "$DATABASE_URL" -X -q -At -F "|" -f -' <<'SQL'
SELECT a.id,
       a.kind,
       a.original_storage_key,
       COALESCE(a.original_size_bytes, -1),
       COALESCE(a.checksum_sha256, ''),
       COALESCE(a.hls_manifest_key, ''),
       COALESCE(a.converted_pdf_key, ''),
       COALESCE(r.processing_status, '')
FROM media_assets a
JOIN resources r ON r.id = a.resource_id
ORDER BY a.created_at;
SQL
