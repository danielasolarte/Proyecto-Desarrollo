<#
Entrega 4 - Terraform con el proyecto y el bucket de estado correctos.

Evita escribir a mano el proyecto y el bucket en cada comando: los toma de
deploy/gcp/e4/proyecto.ps1. Los demas valores (servicios_habilitados) estan
en infra/terraform/variables.tf. Los correos del grupo van en
infra/terraform/terraform.tfvars (lo ignora git), por ejemplo:
  equipo = ["persona1@gmail.com", "persona2@gmail.com"]

Uso:
  .\deploy\gcp\e4\tf.ps1 init
  .\deploy\gcp\e4\tf.ps1 plan                 (solo la base, sin imagenes)
  .\deploy\gcp\e4\tf.ps1 plan -Tag a1b2c3d    (con la etiqueta que imprimio construir-imagenes.ps1)
  .\deploy\gcp\e4\tf.ps1 apply                (aplica el ultimo plan)
  .\deploy\gcp\e4\tf.ps1 output

Siempre plan antes de apply: apply solo aplica el plan guardado, asi que no
cambia nada que no se haya revisado.
#>
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("init", "plan", "apply", "output")]
    [string]$Accion,
    [string]$Tag
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
$chdir = "-chdir=$TF_DIR"

switch ($Accion) {
    "init" {
        terraform $chdir init "-backend-config=bucket=$BUCKET_ESTADO"
    }
    "plan" {
        $argumentos = @($chdir, "plan", "-var=project_id=$P", "-out=tfplan")
        if ($Tag) { $argumentos += "-var=image_tag=$Tag" }
        terraform @argumentos
        if ($LASTEXITCODE -eq 0) {
            Write-Host ""
            Ok "Revisa el resumen (Plan: X to add, Y to change, Z to destroy). Si esta bien: .\deploy\gcp\e4\tf.ps1 apply"
        }
    }
    "apply" {
        if (-not (Test-Path "$TF_DIR/tfplan")) { Falla "No hay plan guardado. Corre primero: .\deploy\gcp\e4\tf.ps1 plan"; return }
        terraform $chdir apply tfplan
        Remove-Item "$TF_DIR/tfplan" -ErrorAction SilentlyContinue
    }
    "output" {
        terraform $chdir output
    }
}
