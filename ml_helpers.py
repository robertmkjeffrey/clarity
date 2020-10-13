from nltk.tokenize.casual import TweetTokenizer, remove_handles 
from bs4 import BeautifulSoup

def text_tokenize(text):
    tknsr = TweetTokenizer()

    raw_tokens = tknsr.tokenize(text)
    tokens = []
    for token in raw_tokens:
        if token.isnumeric(): tokens.append('$NUM$')
        else: tokens.append(token)
    return tokens

def html_tokenize(html):
    raw = BeautifulSoup(html, 'html.parser').text
    
    return text_tokenize(raw)