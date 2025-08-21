# Weather by CEP com observabilidade

<a href="https://www.weatherapi.com/" title="Free Weather API"><img src='//cdn.weatherapi.com/v4/images/weatherapi_logo.png' alt="Weather data by WeatherAPI.com" border="0"></a>

---

Um serviço em Go que fornece a temperatura atual para um determinado CEP (Código de Endereçamento Postal) brasileiro. A aplicação orquestra chamadas a duas APIs externas:

1.  **ViaCEP**: Para encontrar a cidade correspondente ao CEP fornecido.
2.  **WeatherAPI**: Para obter os dados de clima da cidade encontrada.

## ✨ Features

- Validação do formato do CEP (`XXXXX-XXX`).
- Integração com as APIs ViaCEP e WeatherAPI.
- Resposta em JSON com as temperaturas em Celsius, Fahrenheit e Kelvin.
- Pronta para deploy com Docker e otimizada para ambientes como o Google Cloud Run.

## ⚙️ Pré-requisitos

- **Go 1.24** ou superior.
- **Docker** (para execução em contêiner).
- **API Key** do WeatherAPI.com.

## 🚀 Execução Local

1.  **Clone o repositório:**
    ```bash
    git clone <URL_DO_SEU_REPOSITORIO>
    cd <NOME_DA_PASTA>
    ```

2.  **Instale as dependências:**
    ```bash
    go mod tidy
    ```

3.  **Configure a API Key:**
    Crie um arquivo chamado `.env` na raiz do projeto e adicione sua chave da WeatherAPI:
    ```
    WEATHER_API_KEY="sua_chave_api_aqui"
    ```

4.  **Execute a aplicação:**
    ```bash
    go run .
    ```
    O servidor iniciará na porta `8080`.

## 🐳 Execução com Docker

1.  **Construa a imagem Docker:**
    ```bash
    docker build -t weather-by-cep .
    ```

2.  **Execute o contêiner:**
    Substitua `sua_chave_api_aqui` pela sua chave real da WeatherAPI.
    ```bash
    docker run -p 8080:8080 -e WEATHER_API_KEY="sua_chave_api_aqui" weather-by-cep
    ```

## 📡 Uso da API

### Endpoint

`GET /?cep={cep}`

### Parâmetros

- `cep` (string, obrigatório): O CEP no formato `XXXXX-XXX`.

### Exemplo de Requisição

```bash
curl "http://localhost:8080/?cep=01001-000"
```

### Exemplo de Resposta de Sucesso (200 OK)

```json
{
    "city": "São Paulo",
    "temp_C": 21.0,
    "temp_F": 69.8,
    "temp_K": 294.15
}
```

### Respostas de Erro

**`400 Bad Request`**: Se o parâmetro CEP não for fornecido.
```
parâmetro 'cep' é obrigatório
```
**`422 Unprocessable Entity`**: Se o formato do CEP for inválido.
```
CEP inválido
```
**`404 Not Found`**: Se o CEP for válido, mas não for encontrado na API ViaCEP.
```
CEP não encontrado
```
**`500 Internal Server Error`**: Se ocorrer um erro interno no servidor (por exemplo, uma falha ao contatar as APIs externas).
```
ocorreu um erro ao processar sua requisição
```
