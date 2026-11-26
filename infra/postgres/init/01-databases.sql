-- Saybu: inisialisasi database per service.
-- Dijalankan otomatis oleh image Postgres saat volume masih kosong.

CREATE DATABASE saybu_auth;
CREATE DATABASE saybu_catalog;
CREATE DATABASE saybu_shipping;
CREATE DATABASE saybu_orders;
CREATE DATABASE saybu_payments;
CREATE DATABASE saybu_notifications;

\connect saybu_auth
CREATE EXTENSION IF NOT EXISTS citext;

\connect saybu_catalog
-- siap untuk ekstensi tambahan (mis. pg_trgm untuk pencarian)

\connect saybu_shipping

\connect saybu_orders

\connect saybu_payments

\connect saybu_notifications
-- dedicated DB untuk dedup persisten konsumer notifikasi
