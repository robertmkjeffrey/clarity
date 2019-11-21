import sqlite3

# Set up database.
connection = sqlite3.connect("database.db")
with connection:
    connection.execute("CREATE TABLE tweets (id text, author text, content text, has_link text, has_video text, has_image text, is_reply text, is_retweet text, is_quote_rt text, notify text);")
connection.close()