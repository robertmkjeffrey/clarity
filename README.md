# Adopt Detector
This tool is built to help streamline notifications for twitter users. It was inspired by a desire to get notified of artists' commissions without needing to turn on all notifications for the user. The tool works by downloading tweets, and sending a telegram message when a machine learning model classifies the tweet as "notification-worthy". 

## Running

`go run *.go`

## Labelling Instructions
Remember that this tool is aimed to be used with a list of "followed users", rather than just the entirety of twitter's data. As such, tweets should be marked as followed:

> If this tweet came from my favourite user, do I need to be notified immediately?

Some notes:
* Tweets should only be marked if there is a time-critical component of it. Don't mark new items being posted to a site (such as a new book, or t-shirts) unless there's a heavily limited stock of them!
* Raffles will show up anyways if they're good, and usually last for long enough to spread. Label them false!
* Closed adopts / commissions should be marked as False.
