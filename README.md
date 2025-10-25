# S3 Web UI (AWS / Yandex Object Storage)

github: https://github.com/Kohinor46/s3_web_ui/tree/main

Минималистичный веб-интерфейс для просмотра содержимого S3-совместимого хранилища (AWS S3, Yandex Object Storage и др.). Лёгкий одиночный бинарник на Go + один HTML-шаблон. Подходит для self-hosted использования и встраивания во внутренние сервисы.

Demo: список папок и файлов, быстрый фильтр по имени, скачивание файлов.

Возможности
	•	🔎 Быстрый клиентский фильтр по имени (без перезагрузки страницы)
	•	🧭 Навигация по «папкам» (префиксам)
	•	⬇️ Потоковая отдача файлов напрямую из S3
	•	🧰 Совместимость с AWS S3 и Yandex Object Storage (и любыми S3-совместимыми API)
	•	🐳 Готов к контейнеризации (Docker/OCI)
	•	⚙️ Простая конфигурация через переменные окружения/флаги

⸻

Быстрый старт (Go)
./main.go  -s3_user="" -s3_addr="" -s3_bucket="" -s3_pass=""

После запуска откройте: http://localhost:8080/

⸻

Конфигурация

Поддерживаются переменные окружения (удобно для Docker) и/или флаги. Ниже - рекомендуемые имена и значения:

Параметр	Env	Флаг	Описание
Endpoint	s3_addr	-endpoint	Базовый S3-эндпоинт. AWS: https://s3.amazonaws.com, Yandex: https://storage.yandexcloud.net
Access Key	s3_user	—	Ключ доступа
Secret Key	s3_pass	—	Секрет
Bucket	s3_bucket	-bucket	Имя бакета

⸻

Примеры запуска

AWS S3

export s3_user=AKIA...
export s3_pass=...
export s3_bucket=my-bucket

./main.go -s3_addr="https://s3.amazonaws.com" -s3_bucket="$S3_BUCKET"

Yandex Object Storage

export s3_user=YCAJ...
export s3_pass=...
export s3_addr=https://storage.yandexcloud.net
export s3_bucket=my-bucket

./s3-web-ui -endpoint="$s3_addr" -bucket="$s3_bucket"


⸻

Docker

Быстрая команда

docker run --rm -p 8080:8080 \
	-e s3_user="" 
	-e s3_addr="" 
	-e s3_bucket="" 
	-e s3_pass=""
  kohinor46/s3_web_ui:latest

Docker Compose

services:
  s3-web-ui:
    image: kohinor46/s3_web_ui:latest
    ports:
      - "8080:8080"
    environment:
    	s3_user="" 
	s3_addr="" 
	s3_bucket="" 
	s3_pass=""


⸻

Эндпоинты
	•	GET / — листинг текущего префикса (папки)
	•	GET /<subfolder>/ — переход по «папкам»
	•	GET /?<base64encoded-key> — отдача файла (стримом)
	•	GET /favicon.ico — иконка интерфейса (локальный s3.png, если добавлен)

⸻

Защита и доступ

Приложение не реализует аутентификацию. Рекомендуется:
	•	ограничить доступ по IP/VPN; или
	•	повесить Basic Auth/SSO на обратный прокси (nginx/traefik); или
	•	проксировать за API-шлюзом компании.

⸻

Типичные проблемы и решения

SignatureDoesNotMatch
	•	Проверьте регион (AWS_REGION) и эндпоинт (s3_addr).
	•	Убедитесь, что время на сервере синхронизировано (NTP).
	•	Для Yandex Object Storage используйте path-style URL (обычно SDK сам справляется).
	•	Проверьте, что имя бакета без лишних слэшей/пробелов.

AccessDenied
	•	Убедитесь, что у ключей есть права s3:ListBucket и s3:GetObject как минимум.
	•	Для префиксного доступа добавьте условие s3:prefix.

Прокси
	•	Задайте HTTP_PROXY/HTTPS_PROXY для исходящих HTTP(S) соединений к S3.

⸻

Разработка

go run ./s3_web_ui.go \
  -s3_user="" -s3_addr="" -s3_bucket="" -s3_pass=""

Статический шаблон находится в index.gohtml. В нём реализован:
	•	респонсив-верстка,
	•	сортируемая таблица,
	•	клиентский фильтр по имени (без запроса к серверу).

⸻

Лицензия

MIT — используйте свободно. См. файл LICENSE.

⸻

Благодарности
	•	AWS SDK for Go
	•	Всем, кто делает S3-совместимые хранилища удобнее ✨
