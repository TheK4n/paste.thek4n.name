<h1 align="center">Copy/Paste and URL shortener web service</h1>

<p align="center">
  <a href="https://github.com/TheK4n">
    <img src="https://img.shields.io/github/followers/TheK4n?label=Follow&style=social">
  </a>
  <a href="https://github.com/TheK4n/paste.thek4n.name">
    <img src="https://img.shields.io/github/stars/TheK4n/paste.thek4n.name?style=social">
  </a>
</p>

* [Setup](#setup)
* [Usage](#usage)

---

Copy/Paste and URL shortener web service


## Setup

```sh
cd "$(mktemp -d)"
git clone https://github.com/thek4n/paste.thek4n.name .
docker compose up -d
```


## Usage

### Webinterface
http://localhost:8080/


### API

Put text and get it by unique url
```sh
URL="$(curl -d 'Hello' 'localhost:8081/')"
echo "${URL}"  # http://localhost:8081/8fYfLk34Y1H3UQ/
curl "${URL}"  # Hello
```

---

Put text with expiration time
```sh
curl -d 'Hello' 'localhost:8081/?ttl=3h'
curl -d 'Hello' 'localhost:8081/?ttl=30m'
URL="$(curl -d 'Hello' 'localhost:8081/?ttl=60s')"

sleep 61 && curl -i "${URL}"  # 404 Not Found
```

Put disposable text
```sh
URL="$(curl -d 'Hello' 'localhost:8081/?disposable=1')"
curl -i "${URL}"  # Hello
curl -i "${URL}"  # 404 Not Found

```sh
URL="$(curl -d 'Hello' 'localhost:8081/?disposable=2')"
curl -i "${URL}"  # Hello
curl -i "${URL}"  # Hello
curl -i "${URL}"  # 404 Not Found
```

Put URL to redirect
```sh
URL="$(curl -d 'https://example.com/' 'localhost:8081/?url=true')"
curl -iL "${URL}"  # 303 See Other
```

Get clicks
```sh
curl -iL "${URL}/clicks/"  # 1
```

Put disposable url with 3 minute expiration time
```sh
URL="$(curl -d 'https://example.com/' 'localhost:8081/?url=true&disposable=1&ttl=3m')"
curl -iL "${URL}"  # 303 See Other
curl -iL "${URL}"  # 404 Not found
```


<h1 align="center"><a href="#top">▲</a></h1>
