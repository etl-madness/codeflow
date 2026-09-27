--liquibase formatted sql

--changeset architect:orders-v1 context:dev,prod labels:v1.1 runOnChange:false
--comment: Create orders table with foreign keys to customers and merchants
CREATE TABLE orders (
    id BIGINT PRIMARY KEY,
    customer_id BIGINT NOT NULL,
    merchant_id BIGINT NOT NULL,
    order_total DECIMAL(12,2) NOT NULL,
    order_status VARCHAR(30) DEFAULT 'PENDING',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_orders_customer FOREIGN KEY (customer_id) REFERENCES customers(id),
    CONSTRAINT fk_orders_merchant FOREIGN KEY (merchant_id) REFERENCES merchants(id)
);
--rollback DROP TABLE orders;

--changeset architect:payments-v1 context:dev,prod labels:v1.1
--comment: Create payments table with foreign key to orders
CREATE TABLE payments (
    id BIGINT PRIMARY KEY,
    order_id BIGINT NOT NULL,
    payment_method VARCHAR(50) NOT NULL,
    amount DECIMAL(12,2) NOT NULL,
    payment_status VARCHAR(30) DEFAULT 'INITIALIZED',
    transaction_ref VARCHAR(100) UNIQUE,
    processed_at DATETIME,
    CONSTRAINT fk_payments_order FOREIGN KEY (order_id) REFERENCES orders(id)
);
--rollback DROP TABLE payments;

--changeset architect:views-and-procs
--comment: Create analytics view for merchant billing
CREATE VIEW v_merchant_revenue AS
SELECT 
    m.merchant_name,
    COUNT(o.id) AS total_orders,
    SUM(o.order_total) AS gross_revenue
FROM merchants m
JOIN orders o ON m.id = o.merchant_id
WHERE o.order_status = 'COMPLETED'
GROUP BY m.merchant_name;

--changeset architect:milestone-v1.1
--comment: Tag database release milestone
--tag: v1.1-commerce
