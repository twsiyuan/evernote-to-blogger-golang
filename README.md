# Evernote-to-blogger-golang
A small http server helps exports note from evernote to blogger.

## Setup

Setup config.json first

* ```EvernoteAccessToken```: Get Evernote access token using [Developer Tokens](Developer Tokens)
* ```EvernoteClientKey``` and ```EvernoteClientSecret```: Get [Evernote API key](https://dev.evernote.com/#apikey) for prodution
    * Prodution API key is more complicated, see [FAQ: How do I copy my API key from Sandbox to www (production)?](https://dev.evernote.com/support/faq.php#activatekey)
	* Redirect uri: http://127.0.0.1:8080/evernote/callback
	* If you already have ```EvernoteAccessToken```, you can skip this step
* ```GoogleAPIClientID``` and ```GoogleAPIClientSecret```: Your google API project's client id and secret
    * Goto [Google API console](https://console.developers.google.com/apis/)
	* Create new project
	* Enable api: Blogger v3
	* Redirect uri: http://127.0.0.1:8080/blogger/callback
	* Create credential for OAuth2.0
	* Copy client id & secret into config
