from abc import ABC, abstractmethod

class SiteModel(ABC):

    @abstractmethod
    def retrain(self):
        """Retrain the classifier with the most recent data avaliable."""
        pass

    @abstractmethod
    def predict(self, post_id):
        """Predict an element based on an ID."""
        pass

    @abstractmethod
    def getStats(self):
        """Get a set of statistics for the current model."""
        pass

    @abstractmethod
    def getLabelPosts(self):
        """Return a set of posts to be labelled."""
        pass