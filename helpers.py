import sqlite3 # SQLite API
import tweepy # Twitter API
from telebot import types as tbMarkup

def add_tweet(status_id, api):
    """Add a given tweet to the database.
    Inputs:
    =======
    status_id: int or str
        status id of tweet to add to database.
    api: tweepy.API
        twitter API object.
    Returns:
    ========
    row_data: JSON
        row of data extracted from the tweet
    success: bool
        return if the tweet was added (returning false if the tweet was present in the dataset.)
    """
    # TODO: implement
    assert isinstance(status_id, int) or isinstance(status_id, str)
    
    # Get tweet with full text.
    status = api.get_status(status_id, tweet_mode = 'extended')
    row_data = get_row_data(status, get_extended_text=True)

    conn = sqlite3.connect("tweets.db")
    with conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM tweets WHERE id = ?", (status_id,))
        data=cursor.fetchall()
        if len(data)==0:
            conn.execute("INSERT INTO tweets VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", tuple(map(lambda x: str(x) if x is not None else None, row_data.values())))
            success = True
        else:
            success = False
    conn.close()
    return row_data, success
    

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
import emoji
def preprocess(text):
    text = emoji.demojize(remove_handles(text.lower()))
    return text

def tokenize(text):
    tknsr = TweetTokenizer()

    raw_tokens = tknsr.tokenize(text)
    tokens = []
    for token in raw_tokens:
        if token.isnumeric(): tokens.append('$NUM$')
        else: tokens.append(token)
    return tokens
        

def get_row_data(status, get_extended_text=False):
    data = {}
    data['id'] = status.id_str
    data['author'] = status.user.id_str # User Id
    if get_extended_text:
        data['content'] = status.full_text # Tweet text without truncation
    else:
        data['content'] = status.text # Tweet text

    data['has_link'] = len(status.entities['urls']) > 0

    # TODO: check these
    data['has_video'] = 'media' in status.entities and any(map(lambda d: 'video' in d['expanded_url'], status.entities['media']))
    data['has_image'] = not data['has_video'] and 'media' in status.entities and any(map(lambda d: d['type'] == "photo", status.entities['media']))

    data['is_reply'] = status.in_reply_to_user_id is not None
    data['is_retweet'] = "retweeted_status" in status._json
    data['is_quote_rt'] = status.is_quote_status

    data['notify'] = None

    return data