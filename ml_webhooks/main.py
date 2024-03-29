from flask import Flask, request
from pymongo import MongoClient

print("Python: Starting!")

app = Flask(__name__)


#--------------------------------#
# Connect to the MongoDB Database
#--------------------------------#

# Connect to MongoDB
client = MongoClient('mongodb://localhost:27017/')
db_conn = client['adopt-detector-DB']

#--------------------------------#
# Create the site list
#--------------------------------#

from deviantArt_model import DeviantArtModel
# Define a mapping from site names to site objects
SITE_NAMES = {"deviantart": DeviantArtModel(db_conn)}

print("Completed retraining, starting Flask app..")
@app.route('/retrain')
def handle_retrain():
    """Retrain the classifier for the selected site with the most recent data avaliable."""
    site = request.args.get("site")
    if site is None:
        return {"success": False, "error": "Must provide the site to rebuild model for"}

    # Site = "all" means retrain all models.
    if site != "all" and site not in SITE_NAMES.keys():
        # If the site name doesn't exist in the SITE_NAMES dictionary, return an error.
        return {"success": False, "error": f"Cannot find site {site}"}
    
    try:
        # Retrain all models if site == "all". Otherwise retrain the specific site.
        if site == "all":
            [site.retrain() for site in SITE_NAMES.values()]
        else:
            SITE_NAMES[site].retrain()
        return {"success": True, "site" : site}
    except Exception as e:
        return {"success": False, "error": repr(e)}


@app.route('/classify')
def handle_classify():
    """Predict the notification probability of a post."""

    post_id = request.args.get("id")
    if post_id is None:
        return {"success": False, "error":"invalid_request", "error_description":"Must provide an id to be classified."}
    else:
        post_id = str(post_id)

    site = request.args.get("site")
    if site is None:
        return {"success": False, "error":"invalid_request", "error_description":"Must provide the site associated with the id."}

    if site not in SITE_NAMES.keys():
        # If the site name doesn't exist in the SITE_NAMES dictionary, return an error.
        return {"success": False, "error": f"Cannot find site {site}"}
    try:
        return SITE_NAMES[site].predict(post_id)
    except Exception as e:
        return {"success": False, "error": repr(e)}

@app.route('/label')
def handle_label():
    """Get a list of posts to label that will best improve the classifier."""

    count = request.args.get("count")
    if count is None:
        return {"success": False, "error":"invalid_request", "error_description":"Must number of posts to label."}
    count = int(count)

    site = request.args.get("site")
    if site is None:
        return {"success": False, "error":"invalid_request", "error_description":"Must provide the site to get posts for."}

    if site not in SITE_NAMES.keys():
        # If the site name doesn't exist in the SITE_NAMES dictionary, return an error.
        return {"success": False, "error": f"Cannot find site {site}"}
    try:
        return SITE_NAMES[site].getLabelPosts(count)
    except Exception as e:
        return {"success": False, "error": repr(e)}

@app.route('/stats')
def handle_stats():
    site = request.args.get("site")
    if site is None:
        return {"success": False, "error": "Must provide the site to get statistics for"}

    # Site = "all" means retrain all models.
    if site != "all" and site not in SITE_NAMES.keys():
        # If the site name doesn't exist in the SITE_NAMES dictionary, return an error.
        return {"success": False, "error": f"Cannot find site {site}"}
    
    try:
        # Retrain all models if site == "all". Otherwise retrain the specific site.
        if site == "all":
            statistics = "\n".join([site.getStats() for site in SITE_NAMES.values()])
        else:
            statistics = SITE_NAMES[site].getStats()
        return {"success": True, "site" : site, "statistics" : statistics}
    except Exception as e:
        return {"success": False, "error": repr(e)}

@app.route('/status')
def handle_status():
    return "Hello world! \nVersion tag: 00000000000000"

if __name__ == "__main__":

    app.run()