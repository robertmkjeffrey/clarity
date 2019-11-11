# Main Imports
import numpy as np # Arrays
import pandas as pd # Dataframes
import sqlite3 # SQL databases

# Utilities
import joblib

# Helpers
from helpers import tokenize, preprocess

conn = sqlite3.connect("tweets.db")
labelled_tweets = pd.read_sql_query("SELECT * FROM tweets WHERE notify IS NOT NULL", conn)
conn.close()

X = labelled_tweets[['author', 'content', 'has_link', 'has_video', 'has_image']]
y = labelled_tweets['notify']

print(f"""Total Number of Tweets: {len(y)}
Number of notifying tweets: {(y == "True").sum()}
Notification percentage: {(y == "True").sum() / len(y) * 100:.2f}%""")

from sklearn.compose import ColumnTransformer
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import OneHotEncoder
from sklearn.model_selection import cross_val_score
from sklearn.feature_extraction.text import CountVectorizer, TfidfTransformer
from sklearn.naive_bayes import ComplementNB
from sklearn.svm import LinearSVC

categorical_features = ['author', 'has_link', 'has_video', 'has_image']

text_features = 'content'
text_transformer = Pipeline([
        ('vect', CountVectorizer(preprocessor=preprocess, tokenizer=tokenize, ngram_range = (1,2))),
        ('tfidf', TfidfTransformer())
])

                               
preprocessor = ColumnTransformer([
        ('categories', OneHotEncoder(handle_unknown='ignore'), categorical_features),
        ('text', text_transformer, text_features)
])
                                   
clf = Pipeline([
        ('preprocessor', preprocessor),
        ('classifier', LinearSVC(class_weight='balanced'))
])

clf.fit(X, y)

joblib.dump(clf, "model.joblib")

print("Done!")