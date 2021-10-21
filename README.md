# Elvanto overview

A simple web server to provide an overview of volunteers working in upcoming
services who have been scheduled using Elvanto.

!(preview.png)

## Setting up
You'll need access to a superuser account on Elvanto. Register an application as
detailed [https://www.elvanto.com/api/getting-started/#oauth](here), taking note
of the client id and secret. You'll need to set your redirect uri to
https://YOUR_DOMAIN/login/complete.

### Running as Docker container
The Docker image can be pulled from `j18e/elvanto-overview:latest`. When running
you'll need to set a number of environment variables, taken from the Elvanto
integration:
- CLIENT_ID
- CLIENT_SECRET
- REDIRECT_URI

Additional required environment variables:
- DOMAIN for the cookie domain (the same domain as in the redirect URI) eg
  my-site.com
- DATA_FILE is the path to the file you want the bolt database to be stored in.
  This database contains all user token pairs so the app can remember returning
  users and use their tokens for fetching data from Elvanto.

Once it's running you should be able to use it by visiting the external URL
you've set up eg https://my-site.com/.
