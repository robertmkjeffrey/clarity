from flask import Flask, request
app = Flask(__name__)

# Define a mapping from site names to site objects
SITE_NAMES = {}

@app.route('/retrain', methods=['POST'])
def handle_retrain():
    """
    """
    site = request.args.get("site")
    if site is None:
        return {"error":"invalid_request", "error_description":"Must provide the site to rebuild model for."}
    # Rebuild model.
    try:
        SITE_NAMES[site].retrain()
        return {"success": True}
    except KeyError as e:
        # If the site name doesn't exist in the SITE_NAMES dictionary, return an error.
        return {"success": False, "error": "Cannot find site {e.args[0]}"}
    except Exception as e:
        return {"success": False, "error": repr(e)}


@app.route('/classify')
def handle_classify():
    """
    """
    post_id = str(request.args.get("id"))
    if post_id is None:
        return {"error":"invalid_request", "error_description":"Must provide an id to be classified."}
    
    site = request.args.get("site")
    if site is None:
        return {"error":"invalid_request", "error_description":"Must provide the site associated with the id."}

    # TODO: Request data from database
    # TODO: Classify data
    score = 0.69
    notify = True

    return {"id" : post_id, "site": site, "notify": notify, "score" : score}

    try:
        score, notify = SITE_NAMES[site].predict(post_id)
        return {"id" : post_id, "site": site, "notify": notify, "score" : score}
    except KeyError as e:
        # If the site name doesn't exist in the SITE_NAMES dictionary, return an error.
        return {"error": "Cannot find site {e.args[0]}"}
    except Exception as e:
        return {"error": repr(e)}

print("Starting...")

if __name__ == "__main__":
    app.run()