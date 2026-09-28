# Operación y recuperación

## Objetivo

Este documento describe los procedimientos utilizados para operar, respaldar y recuperar la base de datos PostgreSQL administrada en Cloud SQL.

La configuración documentada corresponde al entorno de la Entrega 2.

## Cloud SQL

Proveedor: Google Cloud Platform  
Servicio: Cloud SQL for PostgreSQL  
Región: us-central1  
Versión mayor: PostgreSQL 16  
Tipo de máquina: db-g1-small  
Alta disponibilidad: deshabilitada  
SSL: requerido  

## Migraciones

Las migraciones se encuentran en:

```text
migrations/ 