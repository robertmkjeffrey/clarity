import sqlite3 # SQLite API
from telebot import types as tbMarkup

def labelTweet(tweetId, label):
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

def notify_user(bot, chat_id, tweet_id):
    markup = tbMarkup.InlineKeyboardMarkup()
    markup.row(tbMarkup.InlineKeyboardButton(text="🔗", url=f"https://twitter.com/statuses/{tweet_id}"))
    markup.row(tbMarkup.InlineKeyboardButton(text="✔", callback_data="cb_True"),
                tbMarkup.InlineKeyboardButton(text="❌", callback_data="cb_False"),
                tbMarkup.InlineKeyboardButton(text='🗑️', callback_data="cb_Delete"))
    bot.send_message(chat_id, f"https://twitter.com/statuses/{tweet_id}", reply_markup=markup)