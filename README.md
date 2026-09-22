# کارت به شبا

شماره کارت میدی، بک‌اند از زرین‌هاب شبا رو میگیره و نشون میده.
فرانت به زرین‌هاب وصل نمیشه، فقط به api خودمون درخواست میزنه.

## چی لازم داره

- Go 1.24 به بالا
- Node 20 به بالا
- حساب زرین‌هاب با سرویس CardToIban فعال

## اجرا

دو تا ترمینال جدا. اول بک‌اند، بعد فرانت.

### بک‌اند

تو پوشه `backend` از روی نمونه، فایل env بساز:

```
cd backend
copy .env.example .env
```

داخل `.env` اینا رو از پنل API زرین‌هاب پر کن (لاگین سایت نیست):

- `ZARINHUB_USERNAME` همون GUID نام کاربری API نسخه ۵
- `ZARINHUB_PASSWORD` رمز API که تو پنل ساختی

بقیه مقادیر پیش‌فرض برای لوکال کافیه. فایل env رو گیت نمیکنم.

اگه `go` تو پاورشل شناخته نشد:

```
$env:PATH = "$env:LOCALAPPDATA\Programs\Go\bin;$env:PATH"
```

رو ویندوز Defender گاهی `go run` رو تو Temp گیر میندازه، واسه همین با build اجرا میکنم:

```
go build -o bin\server.exe ./cmd/server
.\bin\server.exe
```

باید ببینی `server listening addr=:8080`

اولین اجرا جدول‌ها رو خودش میسازه (`backend/data`). فقط migration جدا هم میخوای:

```
go run ./cmd/server -migrate-only
```

### فرانت

```
cd frontend
copy .env.example .env
npm install
npm run dev
```

تو `.env` فرانت فقط آدرس بک‌اند هست:

```
NUXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

بعد برو http://localhost:3000
اگه ۳۰۰۰ اشغال بود میره روی ۳۰۰۱. همون رو باز کن.

## api

`POST /api/v1/card-to-iban`

```
curl -X POST http://localhost:8080/api/v1/card-to-iban -H "Content-Type: application/json" -d "{\"card\":\"6037997512345670\"}"
```

سلامت:

```
curl http://localhost:8080/api/v1/health
```

## تست

از پوشه backend، به اکانت زرین‌هاب نیاز نداره:

```
go test ./...
```

## فرضیات / محدودیت

- کارت شتاب ۱۶ رقمه و luhn داره
- بدون لاگین روی api داخلی. هر کی به پورت دسترسی داشته باشه میتونه استعلام بزنه
- rate limit نداره
- اگه وسط استعلام ارتباط قطع بشه وضعیتش رو از زرین‌هاب پیگیری نمیکنم
