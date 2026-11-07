# LLM providers

## List providers

Returns the user's providers ordered by name, keys masked to their last four characters.

`GET /api/providers`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |

**Output**

| Status | Body |
| --- | --- |
| `200` | list of providers |
| `401` | `unauthorized` |

```json
{
  "providers": [
    {
      "id": 1,
      "name": "OpenAI",
      "baseUrl": "https://api.openai.com/v1",
      "keyLast4": "x9Qz",
      "models": ["gpt-4o", "o3"]
    }
  ]
}
```

## Add provider

Adds a provider once `GET {baseUrl}/models` accepts the key and lists every chosen model.

`POST /api/providers`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `name` | body | string | trimmed, 1 to 100 characters, unique among the user's providers |
| `baseUrl` | body | string | absolute http or https URL with no query or fragment, trailing `/` dropped; never a link-local address, loopback or private only when `ALLOW_PRIVATE_PROVIDER_URLS=true` |
| `apiKey` | body | string | trimmed, non-empty, no control characters, accepted by the provider |
| `models` | body | string[] | optional, omitted means none; each listed by the provider, no repeats |

```json
{ "name": "OpenAI", "baseUrl": "https://api.openai.com/v1", "apiKey": "sk-proj-4fT7x9Qz", "models": ["gpt-4o", "o3"] }
```

**Output**

| Status | Body |
| --- | --- |
| `201` | provider, with the models the provider lists |
| `400` | `invalid_name`, `invalid_base_url`, `invalid_api_key`, `provider_unreachable`, `duplicate_model`, `unknown_model`, `bad_request` |
| `401` | `unauthorized` |
| `409` | `name_taken` |

```json
{
  "id": 1,
  "name": "OpenAI",
  "baseUrl": "https://api.openai.com/v1",
  "keyLast4": "x9Qz",
  "models": ["gpt-4o", "o3"],
  "availableModels": ["gpt-4o", "gpt-4o-mini", "o3"]
}
```

## Update provider

Changes a provider's base URL, key or models after fetching `GET {baseUrl}/models` again, or with `{}` only returns the current listing.

`PATCH /api/providers/{providerID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `providerID` | path | integer | one of the user's providers |
| `baseUrl` | body | string | optional; absolute http or https URL with no query or fragment, trailing `/` dropped; never a link-local address, loopback or private only when `ALLOW_PRIVATE_PROVIDER_URLS=true`; a new one needs `apiKey` |
| `apiKey` | body | string | optional, omitted keeps the stored key; trimmed, non-empty, no control characters, accepted by the provider |
| `models` | body | string[] | optional, omitted keeps the stored models; each listed by the provider, no repeats |

```json
{ "baseUrl": "https://api.openai.com/v1", "apiKey": "sk-proj-8kLm2Wv5", "models": ["gpt-4o-mini", "o3"] }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | provider, with the models the provider lists |
| `400` | `invalid_base_url`, `invalid_api_key`, `provider_unreachable`, `duplicate_model`, `unknown_model`, `bad_request` |
| `401` | `unauthorized` |
| `404` | `not_found` |

```json
{
  "id": 1,
  "name": "OpenAI",
  "baseUrl": "https://api.openai.com/v1",
  "keyLast4": "2Wv5",
  "models": ["gpt-4o-mini", "o3"],
  "availableModels": ["gpt-4o", "gpt-4o-mini", "o3"]
}
```

## Delete provider

Removes one of the user's providers.

`DELETE /api/providers/{providerID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `providerID` | path | integer | one of the user's providers |

**Output**

| Status | Body |
| --- | --- |
| `204` | empty |
| `401` | `unauthorized` |
| `404` | `not_found` |
