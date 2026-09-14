-- Development seed data. Explicit IDs + INSERT IGNORE make this safe to re-run.
-- Customer emails are unique, so a pre-existing customer with the same email is left untouched.

INSERT IGNORE INTO customers (id, name, email, phone, address) VALUES
    (1001, 'Acme Corporation',    'billing@acme.example',     '+1-555-0100', '100 Industrial Way, Springfield'),
    (1002, 'Globex Industries',   'accounts@globex.example',  '+1-555-0101', '42 Cypress Creek Rd, Cypress Creek'),
    (1003, 'Initech Software',    'finance@initech.example',  '+1-555-0102', '9 Office Park Dr, Austin'),
    (1004, 'Umbrella Logistics',  'ap@umbrella.example',      '+1-555-0103', '7 Harbor St, Raccoon City');

INSERT IGNORE INTO products (id, name, description, sku, price, stock) VALUES
    (2001, 'Consulting Hour',        'Senior engineering consulting, billed hourly', 'SRV-CONSULT-HR', 150.00, 0),
    (2002, 'Cloud Hosting (Monthly)', 'Managed hosting, standard tier',              'SRV-HOST-STD',   299.00, 0),
    (2003, 'Wireless Keyboard',      'Bluetooth keyboard, US layout',                'HW-KBD-001',      49.99, 120),
    (2004, 'USB-C Dock',             '8-in-1 USB-C docking station',                 'HW-DOCK-008',    129.50, 45),
    (2005, 'Support Plan (Annual)',  'Business-hours support, 12 months',            'SRV-SUPPORT-YR', 1200.00, 0);

-- Totals are stored denormalized; they must equal the sum of the items below plus tax.
INSERT IGNORE INTO invoices
    (id, customer_id, invoice_number, status, issue_date, due_date, subtotal, tax_rate, tax_amount, total, notes)
VALUES
    (3001, 1001, 'INV-SEED-0001', 'paid',  '2026-08-01', '2026-08-31',  899.00, 10.00,  89.90,  988.90, 'Thank you for your business.'),
    (3002, 1002, 'INV-SEED-0002', 'sent',  '2026-09-01', '2026-10-01', 1459.00, 18.00, 262.62, 1721.62, 'Net 30.'),
    (3003, 1003, 'INV-SEED-0003', 'draft', '2026-09-10', '2026-10-10',  279.47,  0.00,   0.00,  279.47, '');

INSERT IGNORE INTO invoice_items (id, invoice_id, product_id, description, quantity, unit_price, line_total) VALUES
    (4001, 3001, 2001, 'Consulting Hour',            4,  150.00,  600.00),
    (4002, 3001, 2002, 'Cloud Hosting (Monthly)',    1,  299.00,  299.00),
    (4003, 3002, 2005, 'Support Plan (Annual)',      1, 1200.00, 1200.00),
    (4004, 3002, 2004, 'USB-C Dock',                 2,  129.50,  259.00),
    (4005, 3003, 2003, 'Wireless Keyboard',          3,   49.99,  149.97),
    (4006, 3003, 2004, 'USB-C Dock',                 1,  129.50,  129.50);
