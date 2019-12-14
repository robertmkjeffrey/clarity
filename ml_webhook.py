from flask import Flask, request
app = Flask(__name__)

@app.route('/retrain', methods=['POST'])
def handle_retrain():
    """
    """
    site = request.args.get("site_name")
    if site is None:
        return {"error":"invalid_request", "error_description":"Must provide the site to rebuild model for."}
    # TODO: Rebuild model.
    return "Success!"


@app.route('/classify')
def handle_classify():
    """
    """
    post_id = request.args.get("id")
    if post_id is None:
        return {"error":"invalid_request", "error_description":"Must provide an id to be classified."}
    
    siteName = request.args.get("site_name")
    if siteName is None:
        return {"error":"invalid_request", "error_description":"Must provide the site associated with the id."}

    # TODO: Request data from database
    # TODO: Classify data
    score = 0.69
    notify = True

    return {"id" : post_id, "site_name": siteName, "notify": notify, "score" : score}

if __name__ == "__main__":
    app.run()