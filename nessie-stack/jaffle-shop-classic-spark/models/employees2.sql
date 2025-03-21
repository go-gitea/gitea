{{
  config(
    materialized='table',
    file_format='iceberg'
  )
}}

-- Simple SELECT to verify that dbt can read from your manual table
-- and write to a new Iceberg table
SELECT * FROM {{ source('test', 'default.employees') }}