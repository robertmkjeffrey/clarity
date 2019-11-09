# Main Modules
import telebot # Telegram API
import sqlite3 # SQLite API
import numpy as np # Arrays
import pandas as pd # Dataframes

# Utility Modules
import yaml
import joblib

# Helpers
from helpers import labelTweet, notify_user

with open("keys.yaml", "r") as f:
    keys = yaml.safe_load(f)

# Create a telegram bot object
bot = telebot.TeleBot(keys['telegram']['api_key'])

# Load machine learning model
clf = joblib.load("model.joblib")

@bot.message_handler(commands=['start'])
def start_handler(message):
    bot.send_message(message.chat.id, "Welcome! This is a bot @DingoDingus uses to get notifed about tweets. If you're him, say /label to start labelling tweets, or wait for notification. If you're not, feel free to DM him about it!")

@bot.message_handler(commands=['label'])
def label_tweets(message):
    num_tweets = 1 if len(message.text.split()) != 2 else int(message.text.split()[1])
    
    # Get list of all non-tagged tweets    
    conn = sqlite3.connect("tweets.db")
    unlabelled_tweets = pd.read_sql_query("SELECT * FROM tweets WHERE notify IS NULL", conn)
    conn.close()
    # Score them and get the ones with the smallest absolute value (closest to decision boundary)
    ul_score = clf.decision_function(unlabelled_tweets)
    # pylint: disable=no-member
    lowest_tweets_indexes = np.argpartition(np.abs(ul_score), num_tweets)[:num_tweets]
    for tweet_index in lowest_tweets_indexes:
        notify_user(bot, keys['telegram']['chat_id'], unlabelled_tweets.loc[tweet_index]['id'], f"Score: {ul_score[tweet_index]:.3f}")
    
@bot.message_handler(commands=['stats'])
def send_stats(message):
    conn = sqlite3.connect('tweets.db')
    with conn:
        unlabelled_count = conn.execute("SELECT count(id) FROM tweets WHERE notify IS NULL").fetchone()[0]
        false_count = conn.execute("SELECT count(id) FROM tweets WHERE notify = 'False'").fetchone()[0]
        true_count = conn.execute("SELECT count(id) FROM tweets WHERE notify = 'True'").fetchone()[0]
    bot.reply_to(message, f"""Bot Version: {0.1}
Total Tweets: {unlabelled_count + false_count + true_count}
Total Labelled Tweets: {false_count + true_count}
Percent Labelled: {((false_count + true_count)/(unlabelled_count + false_count + true_count)) * 100:.2f}%
Notify Percentage: {true_count / (true_count + false_count) * 100:.2f}%"""
)

@bot.message_handler(func=lambda m: True)
def unrecognised_message(message):
    bot.reply_to(message, "Sorry, I didn't understand what you said. Try /help for commands.")

@bot.callback_query_handler(func = lambda x: True)
def callback_handler(call):
    bot.answer_callback_query(call.id)
    # Get the tweet id
    label, tweet_id = call.data.split()

    labelTweet(tweet_id, label)
    # Delete message after labelling
    bot.delete_message(keys['telegram']['chat_id'], call.message.message_id)

try:
    print("Starting polling...")
    bot.polling()
except KeyboardInterrupt as e:
    print("Stopped.")