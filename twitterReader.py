# Main Modules
import tweepy # Twitter API
import telebot # Telegram API
from telebot import types as tbMarkup
import sqlite3 # SQL database management.
import pandas as pd # Dataframes

# Utility Modules
import yaml
from time import sleep
import joblib

# Helpers
from helpers import notify_user, get_row_data

with open("keys.yaml", "r") as f:
    keys = yaml.safe_load(f)

with open("followed_users.yaml", "r") as f:
    followed_users = yaml.safe_load(f)

## SET UP TWITTER API
# Set up the API auth.
auth = tweepy.OAuthHandler(keys['twitter']['api_key'], keys['twitter']['api_secret_key'])
auth.set_access_token(keys['twitter']['access_token'], keys['twitter']['access_token_secret'])

# Create an api object
api = tweepy.API(auth)

# Create a telegram bot object
bot = telebot.TeleBot(keys['telegram']['api_key'])

# Load machine learning model
clf = joblib.load("model.joblib")

# Create a SQLite connection.
con = sqlite3.connect("tweets.db")

# TODO: remove
user_id = followed_users[0]
followed_users = map(str, [user_id] + api.friends_ids(user_id))

class Listener(tweepy.StreamListener):
    """Listens to a tweet stream and responds to each."""
       
    def __init__(self):
        super(Listener,self).__init__()
        self.timeout_counter = 0

    def on_error(self, status_code):
        """Manage error codes by exponentially backing off on timeout, or failing on other errors."""
        # Back off exponentially from 1 minute, doubling each time.
        # https://developer.twitter.com/en/docs/tweets/filter-realtime/guides/connecting
        if status_code == 420:
            sleep_time = 2 ** self.timeout_counter
            self.timeout_counter += 1
            print(f"Timeout hit, sleeping for {sleep_time} minutes...")
            sleep(60 * sleep_time)
            return True
        else:
            print(f"Error code {status_code}, check https://developer.twitter.com/en/docs/tweets/filter-realtime/guides/connecting.")
            return False
    
    def on_status(self, status):
        """React to a tweet by saving any non-reply and non-retween values, and notifying the user if it's desired."""

        row_data = get_row_data(status)
        
        # Remove retweets and replies
        if row_data['is_retweet'] or row_data['is_quote_rt'] or row_data["is_reply"]:
            return True
            
        # If truncated, download it with the extended format
        if status.truncated:
            # Update the status, then update the data
            status = api.get_status(row_data['id'], tweet_mode = 'extended')
            row_data = get_row_data(status, get_extended_text=True)

        # Add the tweet's data the database.
        with con:
            con.execute("INSERT INTO tweets VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", tuple(map(lambda x: str(x) if x is not None else None, row_data.values())))

        print(f"Got tweet from @{status.user.screen_name}.")

        tweet_df = pd.DataFrame.from_dict({k: [str(v)] for k,v in row_data.items()})
        if clf.predict(tweet_df)[0] == 'True':
            print("Sending message...")

            notify_user(bot, keys['telegram']['chat_id'], row_data['id'], f"Score: {clf.decision_function(tweet_df)[0]:.3f}")
        return True
        
# Start listening
listener = Listener()
stream = tweepy.Stream(api.auth, listener, tweet_mode='extended')

try:
    print("Starting stream...")
    stream.filter(follow=followed_users)
except KeyboardInterrupt as e:
    print("Stopped.")
except Exception as e:
    bot.send_message(keys['telegram']['chat_id'], f"Warning: Twitter listener shutting down. Exception: \n{str(e)}")
    with open("twitter.log", 'w') as f:
        f.write(str(e))
    raise e
finally:
    stream.disconnect()
    con.close()
    print('Done.')
