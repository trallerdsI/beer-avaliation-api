-- Reset idempotente: garante que o esquema é recriado do zero pela aplicação
-- (a API é a fonte da verdade). Remove tabelas conhecidas se existirem, antes
-- das migrations de criação. Seguro de correr múltiplas vezes (IF EXISTS).
-- Usado porque o ambiente Supabase pode ficar em estado intermédio se uma
-- migration falhou a meio (ex.: cast SERIAL->UUID).
DROP TABLE IF EXISTS comments CASCADE;
DROP TABLE IF EXISTS beers CASCADE;
DROP TABLE IF EXISTS beerUsers CASCADE;
DROP TABLE IF EXISTS beer_reports CASCADE;
DROP TABLE IF EXISTS beer_deletion_requests CASCADE;
