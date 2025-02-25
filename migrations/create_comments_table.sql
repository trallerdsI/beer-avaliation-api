-- Create the comments table
CREATE TABLE IF NOT EXISTS comments (
    id SERIAL PRIMARY KEY,
    beer_id INT NOT NULL,
    user_id INT NOT NULL,
    text TEXT NOT NULL,
    likes INT DEFAULT 0,
    liked_by TEXT[], -- Array of user IDs who liked the comment
    positive BOOLEAN NOT NULL,
    created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (beer_id) REFERENCES beers(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES beerUsers(id) ON DELETE CASCADE
);