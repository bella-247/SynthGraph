-- Edge cases for the AST translator

-- 1. Composite primary keys
CREATE TABLE composite_pk (
    a INT NOT NULL,
    b INT NOT NULL,
    c VARCHAR(10),
    PRIMARY KEY (a, b)
);

-- 2. Composite foreign keys
CREATE TABLE order_details (
    order_id INT NOT NULL,
    product_id INT NOT NULL,
    detail TEXT,
    PRIMARY KEY (order_id, product_id),
    FOREIGN KEY (order_id, product_id) REFERENCES order_items(order_id, product_id) ON DELETE CASCADE
);

-- 3. Quoted identifiers
CREATE TABLE "MixedCase" (
    "Id" SERIAL PRIMARY KEY,
    "DisplayName" VARCHAR(255)
);

-- 4. Schema-qualified table
CREATE SCHEMA IF NOT EXISTS billing;
CREATE TABLE billing.invoices (
    id SERIAL PRIMARY KEY,
    amount DECIMAL(10, 2) NOT NULL,
    paid BOOLEAN DEFAULT false
);

-- 5. Various data types
CREATE TABLE all_types (
    id INT PRIMARY KEY,
    a_bigint BIGINT,
    a_smallint SMALLINT,
    a_text TEXT,
    a_varchar VARCHAR(255),
    a_char CHAR(10),
    a_boolean BOOLEAN DEFAULT false,
    a_decimal DECIMAL(12, 4),
    a_float FLOAT,
    a_double DOUBLE PRECISION,
    a_date DATE,
    a_time TIME,
    a_timestamp TIMESTAMP,
    a_timestamptz TIMESTAMPTZ,
    a_uuid UUID,
    a_json JSON,
    a_jsonb JSONB,
    a_bytea BYTEA,
    a_interval INTERVAL,
    a_real REAL,
    a_numeric NUMERIC(10, 3)
);

-- 6. Named constraints
CREATE TABLE named_constraints (
    id INT,
    email VARCHAR(255),
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT uq_email UNIQUE (email)
);

-- 7. Self-referencing table
CREATE TABLE employees (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    manager_id INT REFERENCES employees(id),
    department VARCHAR(100)
);

-- 8. Multiple foreign keys on same table
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    author_id INT NOT NULL REFERENCES users(id),
    reviewer_id INT REFERENCES users(id),
    editor_id INT,
    FOREIGN KEY (editor_id) REFERENCES users(id)
);

-- 9. Table with no constraints
CREATE TABLE audit_log (
    id SERIAL,
    event VARCHAR(255),
    payload JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 10. Table with inline DEFAULT expressions
CREATE TABLE default_examples (
    id SERIAL PRIMARY KEY,
    score INT DEFAULT 0,
    ratio FLOAT DEFAULT 1.0,
    is_admin BOOLEAN DEFAULT false,
    label VARCHAR(50) DEFAULT 'untitled',
    created DATE DEFAULT CURRENT_DATE
);

-- 11. FK with ON UPDATE CASCADE
CREATE TABLE configs (
    id INT PRIMARY KEY,
    key VARCHAR(100) NOT NULL
);

CREATE TABLE config_values (
    id INT PRIMARY KEY,
    config_id INT NOT NULL REFERENCES configs(id) ON UPDATE CASCADE ON DELETE CASCADE,
    value TEXT
);

-- 12. Multiple unique constraints
CREATE TABLE multi_unique (
    id INT PRIMARY KEY,
    a VARCHAR(10) UNIQUE,
    b VARCHAR(10) UNIQUE,
    c INT,
    d INT,
    UNIQUE (c, d)
);

-- 13. Composite unique with PK column overlap
CREATE TABLE overlap_unique (
    id INT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    version INT NOT NULL,
    UNIQUE (id, version)
);
