from abstract_site import SiteModel
import pandas as pd

site = "deviantart"
projection = {'url':1, 'title':1, 'description':1, "notify":1, "tags.tag_name":1, "author.username":1}
features = ["author", "title", "description", "tags"]

class DeviantArtModel(SiteModel):
    def __init__(self, db_conn):
        # Set classifier to None.
        self.clf = None
        self.collection = db_conn.deviantartPosts
        pass

    def retrain(self):

        # Download data from MongoDB and convert to ML dataframe.
        raw_data = list(self.collection.find({'notify': {"$exists":True}}, projection))
        df = pd.DataFrame(raw_data)
        df['author']= df['author'].apply(lambda x: x["username"], 1)
        df['tags'] = df['tags'].apply(lambda x: list(map(lambda y: y['tag_name'], x)), 1)
        df.set_index("_id")

        ml_columns = features + ["notify"]
        ml_df = df[ml_columns]

        #------------------------------------------#
        # Train the Model
        #------------------------------------------#
        X = ml_df[features]
        y = ml_df['notify']

        from sklearn.compose import ColumnTransformer
        from sklearn.pipeline import Pipeline
        from sklearn.preprocessing import OneHotEncoder
        from sklearn.model_selection import cross_val_score
        from sklearn.feature_extraction.text import CountVectorizer, TfidfTransformer
        from sklearn.svm import SVC

        from ml_helpers import text_tokenize, html_tokenize

        # The mixed bracketing is here for a reason, I promise
        categorical_features = ['author']
        html_features = 'description'
        list_features = 'tags'
        text_features = 'title'

        text_transformer = Pipeline([
                ('vect', CountVectorizer(tokenizer=text_tokenize, ngram_range = (1,2), min_df=0.2, max_df=0.8)),
                ('tfidf', TfidfTransformer())
        ])

        html_transformer = Pipeline([
                ('vect', CountVectorizer(tokenizer=html_tokenize, ngram_range = (1,2), min_df=0.2, max_df=0.8)),
                ('tfidf', TfidfTransformer())
        ])

        list_transformer = Pipeline([
                ('vect', CountVectorizer(tokenizer=lambda x: x, lowercase=False)),
                ('tfidf', TfidfTransformer())
        ])
            
        preprocessor = ColumnTransformer([
                ('categories', OneHotEncoder(handle_unknown='ignore'), categorical_features),
                ('text', text_transformer, text_features),
                ('list', list_transformer, list_features),
                ('html', html_transformer, html_features)
        ])
                                
        clf = Pipeline([
                ('preprocessor', preprocessor),
                ('classifier', SVC(C=0.5, class_weight='balanced', kernel = 'linear'))
        ])

        # Fit the model to the data.
        clf.fit(X, y)

        # Store the resulting model.
        self.clf = clf

    def predict(self, post_id):
        raw_data = list(self.collection.find({'_id' : post_id}, projection))
        if len(raw_data) == 0:
            return {'success': False, 'site':site, 'id' : post_id, 'error': "id not found in database."}

        post_df = pd.DataFrame(raw_data)
        post_df['author']= post_df['author'].apply(lambda x: x["username"], 1)
        post_df['tags'] = post_df['tags'].apply(lambda x: list(map(lambda y: y['tag_name'], x)), 1)
        post_df.set_index("_id")


        post_ml = post_df[features]
        post_ml

        decision_function = self.clf.decision_function(post_ml)[0]

        return {"success": True, "id" : post_id, "site": site, "notify": bool(decision_function >= 0), "score" : decision_function}

    def getStats(self):
        raise NotImplementedError

    def getLabelPosts(self, count):
        # Get all posts without notify scores.
        raw_data = list(self.collection.find({'notify': {"$exists":False}}, projection))
        df = pd.DataFrame(raw_data)
        df['author']= df['author'].apply(lambda x: x["username"], 1)
        df['tags'] = df['tags'].apply(lambda x: list(map(lambda y: y['tag_name'], x)), 1)
        df.set_index("_id")

        # Keep features as well as the ID to be returned.
        labelling_df = df[features + ["_id"]]

        labelling_df['decision'] = self.clf.decision_function(labelling_df)
        labelling_df['decision_distance'] = labelling_df['decision'].abs()
        
        # Return the IDs of the posts with the `count` smallest distances from the seperating hyperplane.
        ids = list(labelling_df.nsmallest(count, 'decision_distance')['_id'].values)

        return {"success": True, "site": site, "ids": ids}
