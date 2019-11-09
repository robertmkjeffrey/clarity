# Adopt Detector
This tool is built to help streamline notifications for twitter users. It was inspired by a desire to get notified of artists' commissions without needing to turn on all notifications for the user. The tool works by downloading tweets, and sending a telegram message when a machine learning model classifies the tweet as "notification-worthy". 

## Labelling Instructions
Remember that this tool is aimed to be used with a list of "followed users", rather than just the entirety of twitter's data. As such, tweets should be marked as followed:

> If this tweet came from my favourite user, do I need to be notified immediately?

Tweets should only be marked if there is a time-critical component of it.