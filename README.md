Instruções para execução do projeto local com Docker (executar na raiz do projeto):

1. Executar o comando _make docker-compose_
2. Um vez que o container foi criado, enviar um chamada post ao servidor que valida o CEP contendo no corpo da requisição o cep, como no exemplo a seguir:

http://localhost:8080
{
    "cep": "57061-971"
}

Chamadas de exemplo podem ser encontradas no arquivo api/api.http