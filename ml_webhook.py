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

@app.route('/retrain')
def handle_retrain():
    """Retrain the classifier for the selected site with the most recent data avaliable."""
    site = request.args.get("site")
    if site is None:
        return {"success": False, "error":"invalid_request", "error_description":"Must provide the site to rebuild model for."}

    if site not in SITE_NAMES.keys():
        # If the site name doesn't exist in the SITE_NAMES dictionary, return an error.
        return {"success": False, "error": "Cannot find site {e.args[0]}"}
    
    try:
        # Rebuild model.
        SITE_NAMES[site].retrain()
        return {"success": True, "site" : site}
    except Exception as e:
        return {"success": False, "error": repr(e)}


@app.route('/classify')
def handle_classify():
    """Predict the notification probability of a post."""

    print(request.args)

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
        return {"success": False, "error": "Cannot find site {e.args[0]}"}
    try:
        return SITE_NAMES[site].predict(post_id)
    except Exception as e:
        return {"success": False, "error": repr(e)}


# TODO - decide if single call for multiple sites is consistent.
@app.route('/stats')
def handle_stats():
    sites = request.args.get("sites")
    if sites is None:
        return {"success": False, "error": "invalid_request", "error_description":"Must provide the sites to retrieve statistics for. Use \"all\" to get statistics for all sites."}
    
    # TODO: Remove
    print(sites)

    # If all sites are requested, change the list to a list of all sites.
    if sites == "all":
        sites = SITE_NAMES.keys()
    # If only a single site is provided, insert it into a list and continue.
    if type(sites) == str:
        sites = [sites]

    stats = {}
    for site in sites:
        stats[site] = SITE_NAMES[site].getStats()
    
    return stats

@app.route('/status')
def handle_status():
    return "Hello word!"

if __name__ == "__main__":
    app.run()