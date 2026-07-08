-- Create the beers table
CREATE TABLE IF NOT EXISTS beers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    style VARCHAR(50) NOT NULL,
    description TEXT,
    image_url VARCHAR(255),
    alcohol DECIMAL(5, 2),
    taste VARCHAR(50) NOT NULL,
    aroma VARCHAR(50) NOT NULL,
    color VARCHAR(50) NOT NULL,
    body VARCHAR(50) NOT NULL,
    carbonation VARCHAR(50) NOT NULL,
    finish VARCHAR(50) NOT NULL,
    created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
