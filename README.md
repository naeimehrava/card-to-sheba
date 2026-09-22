# کارت به شبا

شماره کارت میدی، بک‌اند از زرین‌هاب شبا رو میگیره و نشون میده.
فرانت به زرین‌هاب وصل نمیشه، فقط به api خودمون درخواست میزنه.

## چی لازم داره

- Go 1.24 به بالا
- Node 20 به بالا
- حساب زرین‌هاب با سرویس CardToIban فعال

غیر از پر کردن یوزرنیم و رمز API تو `.env` کار دیگه‌ای لازم نیست.

## اجرا

دو تا ترمینال جدا. اول بک‌اند، بعد فرانت.

### بک‌اند

```
cd backend
copy .env.example .env
```

تو `.env` فقط این دوتا رو از پنل API زرین‌هاب عوض کن (لاگین سایت و OTP نیست):

- `ZARINHUB_USERNAME` همون GUID نام کاربری API نسخه ۵
- `ZARINHUB_PASSWORD` رمز API که تو پنل ساختی

بقیه مقادیر همون پیش‌فرض `.env.example` برای لوکال کافیه. خود `.env` رو گیت نمیکنم.

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

### فرانت

```
cd frontend
copy .env.example .env
npm install
npm run dev
```

تو `.env` فرانت فقط آدرس بک‌اند هست، پیش‌فرض همینه:

```
NUXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

بعد برو http://localhost:3000
اگه ۳۰۰۰ اشغال بود nuxt میره روی ۳۰۰۱. CORS هر دو پورت رو تو `.env.example` گذاشتم.

## تنظیمات

از env خونده میشه. نمونه بدون secret: `backend/.env.example` و `frontend/.env.example`

| متغیر | پیش‌فرض | چی هست |
| --- | --- | --- |
| `ZARINHUB_USERNAME` | خالی | GUID نام کاربری API |
| `ZARINHUB_PASSWORD` | خالی | رمز API |
| `ZARINHUB_ACCESS_TOKEN` | خالی | اگه پر باشه GetToken نمیره |
| `ZARINHUB_BASE_URL` | `https://zarin-hub.com/api` | آدرس زرین‌هاب |
| `ZARINHUB_TIMEOUT` | `15s` | مهلت هر درخواست بیرونی |
| `ZARINHUB_TOKEN_SKEW` | `60s` | چقدر زودتر از انقضا توکن عوض بشه |
| `HTTP_ADDR` | `:8080` | پورت سرور |
| `HTTP_READ_TIMEOUT` / `WRITE` / `IDLE` | `10s` / `30s` / `60s` | مهلت‌های http |
| `HTTP_SHUTDOWN_TIMEOUT` | `15s` | مهلت خاموش شدن |
| `CORS_ALLOWED_ORIGINS` | `localhost:3000` و `3001` | لیست origin فرانت |
| `TRUST_PROXY_HEADERS` | `false` | فقط پشت پروکسی خودت true کن |
| `DB_DRIVER` | `sqlite` | فعلا همین |
| `DB_DSN` | `file:data/card-to-sheba.db?...` | مسیر دیتابیس |
| `AUDIT_CARD_HASH_SECRET` | خالی | کلید hmac اثر انگشت کارت؛ خالی یعنی خاموش |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `json` | لاگ |
| `NUXT_PUBLIC_API_BASE_URL` | `http://localhost:8080` | آدرس api برای فرانت |

بدون username/password یا توکن ثابت، سرور بالا نمیاد.

## Migration

اسکیما تو `backend/internal/storage/sqlite/migrations/0001_create_inquiries.sql` هست.
با `go:embed` داخل باینریه. اولین `.\bin\server.exe` خودش جدول‌ها رو میسازه (`backend/data`).

فقط migration بدون سرور:

```
cd backend
go run ./cmd/server -migrate-only
```

اگه اینم Defender گرفت، همون `go build` بعد `.\bin\server.exe` کافیه؛ migration همون اول اجرا میشه.

## نمونه API

`POST /api/v1/card-to-iban`

```
curl -X POST http://localhost:8080/api/v1/card-to-iban -H "Content-Type: application/json" -d "{\"card\":\"6037997512345670\"}"
```

موفق:

```
{
  "data": {
    "maskedCard": "603799******5670",
    "iban": "IR...",
    "bankName": "بانک ...",
    "englishBankName": "MELLI",
    "deposit": "...",
    "depositOwners": "...",
    "durationMs": 412
  },
  "requestId": "..."
}
```

خطا:

```
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "شماره کارت معتبر نیست (خطای رقم کنترلی).",
    "fields": [{ "field": "card", "message": "شماره کارت معتبر نیست (خطای رقم کنترلی)." }]
  },
  "requestId": "..."
}
```

کدها: `VALIDATION_ERROR` 400، `CARD_NOT_FOUND` 404، `UPSTREAM_REJECTED` 422، `UPSTREAM_AUTH_ERROR` 502، `UPSTREAM_TIMEOUT` 504، `UPSTREAM_UNAVAILABLE` 503، `INTERNAL_ERROR` 500

سلامت:

```
curl http://localhost:8080/api/v1/health
```

`{"status":"ok"}` یا `{"status":"degraded"}`

## تست

از پوشه backend. به اکانت زرین‌هاب و شبکه بیرون نیاز نداره:

```
cd backend
go test ./...
```

## فرضیات

- سرویس زرین‌هاب همون `POST /api/v5/KYC/CardToIban` با `{"card":"..."}` و توکن از `GetToken` هست
- نام کاربری API یه GUID هست نه موبایل پنل
- کارت شتاب ۱۶ رقمه و luhn داره. BIN چک نمیکنم
- `expiresTn` توکن به ثانیه‌ست
- پاسخ کامل زرین‌هاب تو دیتابیس میره ولی فیلد کارت ماسک میشه؛ شماره کامل ذخیره نمیشه

## محدودیت‌ها

- api داخلی لاگین نداره. هر کی به پورت برسه میتونه استعلام بزنه
- rate limit نداره
- sqlite برای نوشتن تک‌نویسنده‌ست
- اگه وسط استعلام ارتباط قطع بشه از `Tracking/GetStatus` پیگیری نمیکنم
- cache توکن تو حافظه همون پروسه‌ست
- تست خودکار فرانت ندارم
- docker / ci نذاشتم
