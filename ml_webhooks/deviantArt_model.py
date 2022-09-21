from abstract_site import SiteModel
import pandas as pd
import numpy as np

# What percentage of labelling examples should be randomized.
RANDOM_LABELLING_EXAMPLE_RATIO = 0.2

site = "deviantart"
projection = {'url':1, 'title':1, 'description':1, "notify":1, "tags.tag_name":1, "author.username":1}
features = ["author", "title", "description", "tags"]

class DeviantArtModel(SiteModel):
    def __init__(self, db_conn):
        self.collection = db_conn.deviantartPosts
        self.retrain()
        pass

    def _get_DevaintArt_data(self, filter):
        # Download data from MongoDB and convert to ML dataframe.
        raw_data = list(self.collection.find(filter, projection))
        df = pd.DataFrame(raw_data)

        if len(df) == 0:
            return df

        df['author']= df['author'].apply(lambda x: x["username"])
        df['tags'] = df['tags'].apply(lambda x: list(map(lambda y: y['tag_name'], x)))
        df = df.set_index("_id")
        return df

    def retrain(self):

        df = self._get_DevaintArt_data({'notify': {"$exists":True}})

        # If we lack enough data to build a classifier, set a classifier that always returns True.
        if len(df) < 5:

            print(f"Not enough data to train a deviantart model. Require 5 examples, got {len(df)}.")

            class DummyPredictor():
                def __init__(self):
                    pass

                def predict(self, X):
                    try:
                        return [True] * len(X)
                    except:
                        return [True]

                def predict_proba(self, X):
                    try:
                        return [[1, 0]] * len(X)
                    except:
                        return [[1, 0]]

            self.clf = DummyPredictor()
            return

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

        title_transformer = Pipeline([
                ('vect', CountVectorizer(tokenizer=text_tokenize, ngram_range = (1,2), min_df=2, max_df=0.8, stop_words="english")),
                ('tfidf', TfidfTransformer())
        ])

        description_transformer = Pipeline([
                ('vect', CountVectorizer(tokenizer=html_tokenize, ngram_range = (1,2), min_df=2, max_df=0.8, stop_words="english")),
                ('tfidf', TfidfTransformer())
        ])

        tag_transformer = Pipeline([
                ('vect', CountVectorizer(tokenizer=lambda x: x, lowercase=False)),
                ('tfidf', TfidfTransformer())
        ])
            

        # The mixed bracketing for feature names is here for a reason, I promise
        # Update: it's because lists pass a 2d array to the preprocessor,
        # whereas single elements pass a 1d-array.
        # See "columns" in https://scikit-learn.org/stable/modules/generated/sklearn.compose.ColumnTransformer.html
        


        preprocessor = ColumnTransformer([
                # ('categories', OneHotEncoder(handle_unknown='ignore'), categorical_features),
                ('title', title_transformer, "title"),
                ('tags', tag_transformer, "tags"),
                ('description', description_transformer, "description")
        ])
                                
        clf = Pipeline([
                ('preprocessor', preprocessor),
                ('classifier', SVC(C=0.5, class_weight='balanced', kernel = 'linear', probability=True))
        ])

        # Fit the model to the data.
        clf.fit(X, y)

        # Store the resulting model.
        self.clf = clf

    def predict(self, post_id):
        post_df = self._get_DevaintArt_data({'_id' : post_id})

        if len(post_df) == 0:
            return {'success': False, 'site':site, 'id' : post_id, 'error': "id not found in database."}

        if len(post_df) > 1:
            print(f"Warning: non-unqiue id in dataframe. Id: {post_id}")
            return {'success': False, 'site':site, 'id' : post_id, 'error': "id is not unique in database"}


        X_post = post_df[features]

        probability = self.clf.predict_proba(X_post)[0, 0]

        return {"success": True, "id" : post_id, "site": site, "notify": bool(probability >= 0), "score" : probability}

    def getStats(self):

        df = self._get_DevaintArt_data({})

        # Aggregate author statistics into dataframe.
        author_df = df.author.value_counts()[:5].to_frame("Post Count")
        author_df["Labelled Rate"] = df.groupby("author").apply(lambda x: (~x.notify.isna()).mean())
        author_df["Notification Rate"] = df.groupby("author").apply(lambda x: x[~x.notify.isna()].notify.mean())
        author_df_string = (author_df.to_string(formatters= {
            'Labelled Rate': '{:,.2%}'.format,
            'Notification Rate': '{:,.2%}'.format,
        }))


        statistics = f"""Number of Posts: {len(df)}
        Percent Labelled: {(~df.notify.isna()).mean() * 100:.2f}%
        Notification Rate: {df[~df.notify.isna()].notify.mean() * 100:.2f}%\n
        Top-5 Author Statistics:
        {author_df_string}"""

        return statistics

    def getLabelPosts(self, count):

        if self.clf is None:
            return {"success": False}

        # Get all posts without notify scores.
        df = self._get_DevaintArt_data({'notify': {"$exists":False}})
        # Keep features as well as the ID to be returned.
        labelling_df = df[features]

        labelling_df['probability'] = self.clf.predict_proba(labelling_df)[:,1]
        labelling_df['decision_distance'] = (labelling_df['probability'] - 0.5).abs()
        
        # Return the IDs of the posts with the `count` smallest distances from the seperating hyperplane.
        # Sample first to prevent bias towards earlier posts in dataframe.
        ids = labelling_df.sample(frac=1).nsmallest(count, 'decision_distance').index.values

        # Randomly select indicies to fill with random posts.
        # This prevents an inductive meltdown where confident mistakes aren't re-assessed.
        randomisation_index = np.random.random((len(ids))) < RANDOM_LABELLING_EXAMPLE_RATIO
        random_ids = labelling_df.sample(len(ids)).index

        ids[randomisation_index] = random_ids[randomisation_index]

        return {"success": True, "site": site, "ids": list(ids)}
