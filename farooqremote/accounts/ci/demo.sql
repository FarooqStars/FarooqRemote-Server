-- Farooq Remote (FA-020): demo data for the build's pictures only (never on the real server)
INSERT INTO peers (id, cpu, hostname, memory, os, username, uuid, version, user_id, last_online_time, last_online_ip, group_id, alias, created_at, updated_at) VALUES
 ('123456789', 'Intel Core i7-12700', 'HOME-PC1', '32 GB', 'Windows 11 Pro', 'farooq', 'demo-pc1', '1.4.9', 1, strftime('%s','now'), '10.0.0.11', 1, 'PC1', datetime('now','-30 days'), datetime('now')),
 ('234567891', 'Intel Core i5-11400', 'HOME-PC2', '16 GB', 'Windows 11 Pro', 'farooq', 'demo-pc2', '1.4.9', 1, strftime('%s','now') - 300, '10.0.0.12', 1, 'PC2', datetime('now','-30 days'), datetime('now')),
 ('345678912', 'AMD Ryzen 7 5800U', 'LAPTOP', '16 GB', 'Windows 11 Home', 'farooq', 'demo-laptop', '1.4.9', 1, strftime('%s','now') - 90000, '10.0.0.13', 1, 'Laptop', datetime('now','-20 days'), datetime('now')),
 ('456789123', 'Intel Core i5-10500', 'OFFICE-PC', '8 GB', 'Windows 10 Pro', 'farooq', 'demo-office', '1.4.9', 1, strftime('%s','now') - 4000, '10.0.0.14', 1, 'Office', datetime('now','-10 days'), datetime('now'));
INSERT INTO tags (name, user_id, color, collection_id, created_at, updated_at) VALUES
 ('Home', 1, 4280391411, 0, datetime('now'), datetime('now')),
 ('Office', 1, 4294940672, 0, datetime('now'), datetime('now'));
INSERT INTO address_books (id, username, password, hostname, alias, platform, tags, hash, user_id, force_always_relay, rdp_port, rdp_username, online, login_name, same_server, collection_id, created_at, updated_at) VALUES
 ('123456789', 'farooq', '', 'HOME-PC1', 'PC1', 'Windows', '["Home"]', '', 1, 0, '', '', 1, '', 0, 0, datetime('now'), datetime('now')),
 ('234567891', 'farooq', '', 'HOME-PC2', 'PC2', 'Windows', '["Home"]', '', 1, 0, '', '', 1, '', 0, 0, datetime('now'), datetime('now')),
 ('345678912', 'farooq', '', 'LAPTOP', 'Laptop', 'Windows', '["Home"]', '', 1, 0, '', '', 0, '', 0, 0, datetime('now'), datetime('now')),
 ('456789123', 'farooq', '', 'OFFICE-PC', 'Office', 'Windows', '["Office"]', '', 1, 0, '', '', 1, '', 0, 0, datetime('now'), datetime('now'));
INSERT INTO login_logs (user_id, client, device_id, uuid, ip, type, platform, user_token_id, is_deleted, created_at, updated_at) VALUES
 (1, 'app', '123456789', 'demo-pc1', '10.0.0.11', 'account', 'windows', 0, 0, datetime('now','-2 hours'), datetime('now','-2 hours')),
 (1, 'app', '345678912', 'demo-laptop', '10.0.0.13', 'account', 'windows', 0, 0, datetime('now','-1 days'), datetime('now','-1 days')),
 (1, 'webadmin', '', '', '10.0.0.11', 'account', '', 0, 0, datetime('now','-10 minutes'), datetime('now','-10 minutes'));
