from typing import List, Tuple, Dict
from abc import ABC, abstractmethod

class SiteModel(ABC):
    @abstractmethod
    def __init__(self, db_conn):
        """Initialise site module."""
        self.retrain()
        pass

    @abstractmethod
    def retrain(self):
        """Retrain the classifier with the most recent data avaliable."""
        pass

    @abstractmethod
    def predict(self, post_id: str) -> Tuple[float, bool]:
        """Predict an element based on an ID.
        
        Inputs:
        =======
            post_id: str
        
        Returns:
        ========
            (score, notify): (float, bool)
                a score for the post representing the probability of notification, and a boolean representing whether to notify or not."""
        
        pass

    @abstractmethod
    def getStats(self) -> Dict[str, str]:
        """Get a set of statistics for the current model."""
        pass

    @abstractmethod
    def getLabelPosts(self, count: int) -> List[str] :
        """Return a set of posts to be labelled.
        
        Inputs:
        =======
            count: int
            number of posts to label.

        Returns:
        ========
            labelPosts: list(str)"""
        pass