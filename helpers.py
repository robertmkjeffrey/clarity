import sqlite3 # SQLite API
import tweepy # Twitter API
from telebot import types as tbMarkup

def add_tweet(status):
    """Add a given tweet to the database.
    Inputs:
    =======
    status: tweepy.Status
        status to add to database.
    """
    # TODO: implement
    assert isinstance(status, tweepy.Status)
    raise NotImplementedError

def label_tweet(tweetId, label):
    """Set the notify label of a tweet to a given label.
    Inputs:
    =======

    tweetId: int or str
        id of the tweet to update.

    label: bool or None
        new label for the tweet.
    """
    # Make sure label is of valid type
    assert (label == "cb_True" or label == "cb_False" or label=="cb_Delete"), f"Cannot use label setting {label}"
    connection = sqlite3.connect("tweets.db")
    with connection:
        if label == 'cb_True' or label == 'cb_False':
            connection.execute("UPDATE tweets SET notify = ? WHERE id = ?", (label[3:], str(tweetId)))
        elif label == 'cb_Delete':
            connection.execute("DELETE FROM tweets WHERE id = ?", (str(tweetId),))
    connection.close()

def notify_user(bot, chat_id, tweet_id, message=""):
    if message:
        message = message + '\n' + f"https://twitter.com/statuses/{tweet_id}"
    else:
        message = f"https://twitter.com/statuses/{tweet_id}"

    markup = tbMarkup.InlineKeyboardMarkup()
    markup.row(tbMarkup.InlineKeyboardButton(text="🔗", url=f"https://twitter.com/statuses/{tweet_id}"))
    markup.row(tbMarkup.InlineKeyboardButton(text="✔", callback_data=f"cb_True {tweet_id}"),
                tbMarkup.InlineKeyboardButton(text="❌", callback_data=f"cb_False {tweet_id}"),
                tbMarkup.InlineKeyboardButton(text='🗑️', callback_data=f"cb_Delete {tweet_id}"))
    bot.send_message(chat_id, message, reply_markup=markup)
    

from nltk.tokenize.casual import TweetTokenizer, remove_handles 
def tokenise(text):
    tknsr = TweetTokenizer()
    
    text = remove_handles(text).lower()
    raw_tokens = tknsr.tokenize(text)
    tokens = []
    for token in raw_tokens:
        if token.isnumeric(): tokens.append('$NUM$')
        else: tokens.append(token)
    return tokens