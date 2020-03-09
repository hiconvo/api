module github.com/hiconvo/api

require (
	cloud.google.com/go v0.43.0
	github.com/GetStream/stream-go2 v3.2.1+incompatible // indirect
	github.com/PaesslerAG/gval v1.0.1 // indirect
	github.com/PuerkitoBio/goquery v1.5.0 // indirect
	github.com/arran4/golang-ical v0.0.0-20191011054615-fb8af82a1cf8
	github.com/aymerick/douceur v0.2.0
	github.com/certifi/gocertifi v0.0.0-20190506164543-d2eda7129713 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/getsentry/raven-go v0.2.0
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/gorilla/css v1.0.0 // indirect
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.2
	github.com/gosimple/slug v1.5.0
	github.com/imdario/mergo v0.3.7
	github.com/jaytaylor/html2text v0.0.0-20190408195923-01ec452cbe43
	github.com/mailru/easyjson v0.0.0-20190626092158-b2ccc519800e // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/microcosm-cc/bluemonday v1.0.2
	github.com/olekukonko/tablewriter v0.0.1 // indirect
	github.com/olivere/elastic/v7 v7.0.3
	github.com/otiai10/opengraph v1.1.0
	github.com/rainycape/unidecode v0.0.0-20150907023854-cb7f23ec59be // indirect
	github.com/russross/blackfriday/v2 v2.0.1
	github.com/sendgrid/rest v2.4.1+incompatible
	github.com/sendgrid/sendgrid-go v3.4.1+incompatible
	github.com/sergi/go-diff v1.0.0 // indirect
	github.com/ssor/bom v0.0.0-20170718123548-6386211fdfcf // indirect
	github.com/steinfletcher/apitest v1.4.0
	github.com/steinfletcher/apitest-jsonpath v1.3.2
	github.com/stretchr/testify v1.4.0
	gocloud.dev v0.15.0
	golang.org/x/crypto v0.0.0-20190605123033-f99c8df09eb5
	golang.org/x/sys v0.0.0-20200302150141-5c8b2ff67527 // indirect
	golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898 // indirect
	google.golang.org/api v0.10.0
	google.golang.org/genproto v0.0.0-20190716160619-c506a9f90610
	googlemaps.github.io/maps v0.0.0-20190906051648-24f4c8471353
	gopkg.in/GetStream/stream-go2.v1 v1.14.0
	gopkg.in/LeisureLink/httpsig.v1 v1.2.0 // indirect
	gopkg.in/dgrijalva/jwt-go.v3 v3.2.0 // indirect
	gopkg.in/validator.v2 v2.0.0-20180514200540-135c24b11c19
	gopkg.in/yaml.v2 v2.2.5 // indirect
	mvdan.cc/xurls/v2 v2.1.0
)

replace gopkg.in/russross/blackfriday.v2 => github.com/russross/blackfriday/v2 v2.0.1

go 1.13
