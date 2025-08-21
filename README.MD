# Desafio de Observabilidade com OpenTelemetry

Este projeto consiste em um sistema distribuído de dois serviços em Go que, juntos, fornecem a temperatura atual de uma localidade a partir de um CEP (Código de Endereçamento Postal). Toda a comunicação entre os serviços e com as APIs externas é instrumentada com **OpenTelemetry** para garantir observabilidade completa, com traces visualizados no **Jaeger**.

<p align="center">
  <img src="https://opentelemetry.io/img/logos/opentelemetry-logo-nav.png" width="300" alt="OpenTelemetry Logo">
  &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="https://www.jaegertracing.io/img/jaeger-logo-color.png" width="200" alt="Jaeger Logo">
</p>

---

## Arquitetura

O sistema é composto por dois microserviços e o Jaeger, orquestrados via Docker Compose:

1.  **`app1` (Serviço de Entrada)**:
    *   Recebe as requisições do cliente com o CEP.
    *   Valida o formato do CEP.
    *   Chama o `app2` para obter os dados de clima.
    *   É a porta de entrada do sistema, escutando na porta `8080`.

2.  **`app2` (Serviço Orquestrador)**:
    *   Recebe o CEP validado do `app1`.
    *   Consulta a API **ViaCEP** para obter a cidade correspondente.
    *   Consulta a API **WeatherAPI** para obter a temperatura da cidade.
    *   Retorna os dados consolidados para o `app1`.

3.  **`jaeger`**:
    *   Recebe e armazena os dados de telemetria (traces) enviados pelos serviços via OTLP.
    *   Fornece uma interface web para visualização e análise dos traces distribuídos.

O fluxo da requisição é o seguinte:
`Cliente -> app1 -> app2 -> (ViaCEP & WeatherAPI) -> app2 -> app1 -> Cliente`

## ✨ Features

- **Sistema Distribuído**: Arquitetura baseada em dois microserviços Go.
- **Observabilidade Completa**: Instrumentação com OpenTelemetry e visualização de traces no Jaeger.
- **Validação de CEP**: Garante que apenas CEPs no formato `XXXXX-XXX` sejam processados.
- **Orquestração de APIs**: Integração com ViaCEP e WeatherAPI.
- **Resposta em JSON**: Retorna a cidade e as temperaturas em Celsius, Fahrenheit e Kelvin.
- **Containerização**: Pronto para deploy com Docker e Docker Compose.

## ⚙️ Pré-requisitos

- **Go 1.24** ou superior (para execução local).
- **Docker** e **Docker Compose**.
- **API Key** do WeatherAPI.com.

## 🚀 Execução

Existem duas maneiras principais de executar o projeto: a forma recomendada, utilizando Docker Compose para toda a stack, ou um modo híbrido para desenvolvimento.

### Método 1: Docker Compose (Recomendado)

Esta é a forma mais simples de subir todos os serviços (`app1`, `app2`, e `jaeger`) de uma vez.

1.  **Clone o repositório:**
    ```bash
    git clone git@github.com:psaraiva/l02.git
    cd l02
    ```

3.  **Configure a API Key:**
    Crie um arquivo chamado `.env` dentro da pasta `app2` (`app2/.env`) e adicione sua chave da WeatherAPI:
    ```
    WEATHER_API_KEY="sua_chave_api_aqui"
    ```

4.  **Execute a stack com Docker Compose:**
    ```bash
    docker-compose up -d
    ```
    Os serviços estarão disponíveis nos seguintes endereços:
    - **Serviço de Clima (`app1`)**: `http://localhost:8080`
    - **Serviço Orquestrador (`app2`)**: `http://localhost:8083`
    - **Jaeger UI**: `http://localhost:16686`

### Método 2: Híbrido (Go Local + Jaeger em Docker)

Este método é ideal para desenvolvimento, pois permite executar o código Go localmente enquanto utiliza o Jaeger via Docker.

1.  **Clone o repositório e configure a API Key** conforme os passos 1 e 2 do método anterior.

2.  **Inicie o Jaeger com Docker Compose:**
    ```bash
    docker-compose up -d jaeger
    ```
    Isso iniciará apenas o contêiner do Jaeger.

3.  **Execute o `app2` (Serviço Orquestrador):**
    Abra um novo terminal, navegue até a pasta do `app2` e execute:
    ```bash
    cd app2
    go run main.go
    ```
    O serviço `app2` iniciará na porta `8083`.

4.  **Execute o `app1` (Serviço de Entrada):**
    Abra outro terminal, navegue até a pasta do `app1` e execute:
    ```bash
    cd app1
    go run main.go
    ```
    O serviço `app1` iniciará na porta `8080`.

## 📡 Uso da API

As requisições devem ser feitas para o `app1`.

### Endpoint

`POST /weather-by-cep`

### Corpo da Requisição (JSON)

```json
{
    "cep": "01001-000"
}
```

### Exemplo de Requisição com `curl`

```bash
curl --location 'http://localhost:8080/weather-by-cep' \
--header 'Content-Type: application/json' \
--data '{
    "cep": "01001-000"
}'
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

## 🧐 Jaeger

A instrumentação com OpenTelemetry é um dos pilares deste projeto, permitindo visualizar o ciclo de vida completo de uma requisição em um **trace distribuído**. Isso é fundamental para depurar e entender a performance do sistema, mostrando como uma única chamada na `app1` se propaga pela `app2` até as APIs externas.

A imagem abaixo ilustra este fluxo, detalhando cada etapa e sua respectiva latência:

![Exemplo de Trace Distribuído no Jaeger](doc/jaeger-example.png)
