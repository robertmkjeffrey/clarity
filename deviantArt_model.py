from abstract_site import SiteModel
class DeviantArtModel(SiteModel):
    def __init__(self):
        pass

    def retrain(self):
        raise NotImplementedError

    def predict(self, post_id):
        return 0.69, True

    def getStats(self):
        raise NotImplementedError

    def getLabelPosts(self):
        raise NotImplementedError