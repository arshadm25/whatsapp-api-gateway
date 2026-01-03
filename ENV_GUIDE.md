# How to Get WhatsApp API Credentials

Here is where you find each value for your `.env` file:

## 1. `VERIFY_TOKEN`
*   **Value:** `my_secret_verify_token` (or any string you choose).
*   **Where to use it:** You create this yourself. You will paste this SAME string into the **Meta App Dashboard > WhatsApp > Configuration > Webhook** section.

## 2. `PHONE_NUMBER_ID` and `WABA_ID`
*   **Go to:** [Meta App Dashboard](https://developers.facebook.com/apps/) -> Your App -> **WhatsApp** -> **API Setup**.
*   **Look for:**
    *   **Phone Number ID**: Listed under the "From" phone number dropdown.
    *   **WhatsApp Business Account ID** (WABA_ID): Listed right above the Phone Number ID.

## 3. `WHATSAPP_TOKEN` (Access Token)
*   **For Testing (24 hours):**
    *   Go to **WhatsApp** -> **API Setup**.
    *   Copy the **Temporary Access Token**.
*   **For Production (Permanent):**
    1.  Go to [Business Settings](https://business.facebook.com/settings).
    2.  Navigate to **Users** -> **System Users**.
    3.  Add a user (Role: Admin).
    4.  Click **Generate New Token**.
    5.  Select your App.
    6.  Select Permissions: `whatsapp_business_messaging`, `whatsapp_business_management`.
    7.  Copy the generated token.

## Summary `.env`
```bash
PORT=8080
VERIFY_TOKEN=my_secret_verify_token
WHATSAPP_TOKEN=EAAG... (The long string starting with EAA)
PHONE_NUMBER_ID=109... (Usually starts with 1)
WABA_ID=112... (Usually starts with 1, needed for Templates)
```
