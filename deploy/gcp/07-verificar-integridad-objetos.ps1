<#
Correccion C5 de la Entrega 2: verificar la integridad de los objetos migrados
a Cloud Storage y su correspondencia con las referencias en PostgreSQL.

Que comprueba, por cada fila de media_assets:
  1. El original existe en el bucket y su tamano coincide con original_size_bytes.
  2. Para los primeros -MaxHash originales con checksum_sha256, descarga el
     objeto y compara su SHA-256 con el guardado en la base de datos.
  3. Si el recurso esta listo y tiene manifiesto HLS, el .m3u8 existe y hay
     segmentos .ts/.m4s bajo el mismo prefijo.
  4. Si tiene PDF convertido, el objeto existe.
Ademas cuenta los objetos del bucket que ninguna fila referencia (huerfanos).

Uso:
  . .\deploy\gcp\00-variables.ps1
  .\deploy\gcp\07-verificar-integridad-objetos.ps1 -Bucket mooc-e2-media-proyecto1-entrega2

Resultado: docs/entrega2/integridad-objetos.md (sin secretos).
#>
param(
    [string]$Bucket = "mooc-e2-media-proyecto1-entrega2",
    [int]$MaxHash = 10,
    [string]$Salida = "docs/entrega2/integridad-objetos.md"
)

$ErrorActionPreference = "Continue"
# Se puede lanzar con Terminal > Run Active File: se ubica en la raiz del repo,
# carga las variables si faltan y deja el registro en deploy/gcp/logs/.
Set-Location (Resolve-Path (Join-Path $PSScriptRoot "..\.."))
if (-not $env:WEB_VM) { . (Join-Path $PSScriptRoot "00-variables.example.ps1") }
New-Item -ItemType Directory -Force "deploy/gcp/logs" | Out-Null
Start-Transcript -Path "deploy/gcp/logs/07-integridad-$(Get-Date -Format 'yyyyMMdd-HHmmss').log" | Out-Null
$gc = @("--zone=$env:GCP_ZONE", "--project=$env:GCP_PROJECT", "--tunnel-through-iap", "--strict-host-key-checking=no")

Write-Host "1/4 Consultando media_assets en Cloud SQL desde $($env:WEB_VM)..."
gcloud compute scp deploy/gcp/remoto/consulta-media-assets.sh "$($env:WEB_VM):/tmp/consulta-media-assets.sh" @gc 2>&1 | Out-Null
$filas = gcloud compute ssh $env:WEB_VM @gc --command="bash /tmp/consulta-media-assets.sh" 2>$null |
    Where-Object { $_ -match "^[0-9a-f-]{36}\|" }
$assets = foreach ($f in $filas) {
    $c = $f -split "\|"
    [pscustomobject]@{ Id = $c[0]; Kind = $c[1]; Key = $c[2]; Size = [int64]$c[3]; Sha = $c[4].ToLower() -replace "^sha256:", ""; Hls = $c[5]; Pdf = $c[6]; Estado = $c[7] }
}
Write-Host "    $($assets.Count) filas en media_assets"

Write-Host "2/4 Listando objetos de gs://$Bucket ..."
$objetos = @{}
gcloud storage objects list "gs://$Bucket/**" --project=$env:GCP_PROJECT --format="csv[no-heading](name,size)" 2>$null | ForEach-Object {
    $p = $_ -split ","
    if ($p.Count -ge 2) { $objetos[$p[0]] = [int64]$p[1] }
}
Write-Host "    $($objetos.Count) objetos en el bucket"
$nombres = @($objetos.Keys)

Write-Host "3/4 Comparando referencias..."
$tmp = Join-Path $env:TEMP "mooc-integridad"
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$hashes = 0
$referenciados = New-Object System.Collections.Generic.HashSet[string]
$resultado = foreach ($a in $assets) {
    $problemas = @()
    $original = "falta"
    if ($objetos.ContainsKey($a.Key)) {
        [void]$referenciados.Add($a.Key)
        $original = "ok"
        if ($a.Size -ge 0 -and $objetos[$a.Key] -ne $a.Size) { $problemas += "tamano BD $($a.Size) vs bucket $($objetos[$a.Key])" }
    } else { $problemas += "original no existe" }

    $sha = "no verificado"
    if ($original -eq "ok" -and $a.Sha -and $hashes -lt $MaxHash) {
        $local = Join-Path $tmp ($a.Id + ".bin")
        gcloud storage cp "gs://$Bucket/$($a.Key)" $local --quiet 2>$null | Out-Null
        if (Test-Path $local) {
            $real = (Get-FileHash -Algorithm SHA256 -Path $local).Hash.ToLower()
            Remove-Item $local -ErrorAction SilentlyContinue
            $hashes++
            if ($real -eq $a.Sha) { $sha = "coincide" } else { $sha = "NO coincide"; $problemas += "sha256 distinto" }
        } else { $sha = "no se pudo descargar" }
    }

    $hls = "-"
    if ($a.Hls) {
        $prefijo = $a.Hls.Substring(0, $a.Hls.LastIndexOf("/") + 1)
        $segmentos = @($nombres | Where-Object { $_.StartsWith($prefijo) -and ($_ -match "\.(ts|m4s)$") })
        $nombres | Where-Object { $_.StartsWith($prefijo) } | ForEach-Object { [void]$referenciados.Add($_) }
        if ($objetos.ContainsKey($a.Hls) -and $segmentos.Count -gt 0) { $hls = "ok ($($segmentos.Count) segmentos)" }
        else { $hls = "incompleto"; $problemas += "HLS sin manifiesto o sin segmentos" }
    } elseif ($a.Estado -eq "ready" -and ($a.Kind -eq "video" -or $a.Kind -eq "audio")) {
        $hls = "falta"; $problemas += "recurso ready sin manifiesto HLS"
    }

    $pdf = "-"
    if ($a.Pdf) {
        if ($objetos.ContainsKey($a.Pdf)) { $pdf = "ok"; [void]$referenciados.Add($a.Pdf) } else { $pdf = "falta"; $problemas += "PDF convertido no existe" }
    }

    [pscustomobject]@{ Id = $a.Id; Kind = $a.Kind; Estado = $a.Estado; Original = $original; Sha = $sha; Hls = $hls; Pdf = $pdf; Problemas = ($problemas -join "; ") }
}
$huerfanos = @($nombres | Where-Object { -not $referenciados.Contains($_) })

Write-Host "4/4 Escribiendo $Salida"
$conProblemas = @($resultado | Where-Object { $_.Problemas })
$commit = (git rev-parse --short HEAD)
$lineas = @()
$lineas += "# Verificacion de integridad de objetos migrados (Entrega 2, correccion C5)"
$lineas += ""
$lineas += "- Fecha: $(Get-Date -Format 'yyyy-MM-dd HH:mm') (hora local)"
$lineas += "- Commit: ``$commit``"
$lineas += "- Bucket: ``gs://$Bucket``"
$lineas += "- Script: ``deploy/gcp/07-verificar-integridad-objetos.ps1``"
$lineas += ""
$lineas += "## Resumen"
$lineas += ""
$lineas += "- Filas en ``media_assets``: $($assets.Count)"
$lineas += "- Objetos en el bucket: $($objetos.Count)"
$lineas += "- Originales presentes con tamano correcto: $(@($resultado | Where-Object { $_.Original -eq 'ok' -and $_.Problemas -notmatch 'tamano' }).Count) de $($assets.Count)"
$lineas += "- SHA-256 verificados descargando el objeto: $hashes (coinciden: $(@($resultado | Where-Object { $_.Sha -eq 'coincide' }).Count))"
$lineas += "- Filas con algun problema: $($conProblemas.Count)"
$lineas += "- Objetos sin referencia en la base de datos: $($huerfanos.Count)"
$lineas += ""
$lineas += "## Detalle por recurso"
$lineas += ""
$lineas += "| media_asset | Tipo | Estado | Original | SHA-256 | HLS | PDF | Problemas |"
$lineas += "| --- | --- | --- | --- | --- | --- | --- | --- |"
foreach ($r in $resultado) {
    $lineas += "| ``$($r.Id.Substring(0,8))`` | $($r.Kind) | $($r.Estado) | $($r.Original) | $($r.Sha) | $($r.Hls) | $($r.Pdf) | $($r.Problemas) |"
}
if ($huerfanos.Count -gt 0) {
    $lineas += ""
    $lineas += "## Objetos sin referencia (primeros 30)"
    $lineas += ""
    $huerfanos | Select-Object -First 30 | ForEach-Object { $lineas += "- ``$_``" }
}
$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllLines((Join-Path (Get-Location) $Salida), $lineas, $utf8)
Write-Host "Listo. Filas con problemas: $($conProblemas.Count). Huerfanos: $($huerfanos.Count)."
Stop-Transcript | Out-Null
