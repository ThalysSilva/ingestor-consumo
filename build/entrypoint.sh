#!/bin/bash
set -e
set -x  # Ativa depuração

# Aguarda até que redis-primary esteja disponível
until redis-cli -h redis-primary -p 6379 ping; do
  echo "Aguardando redis-primary estar disponível..."
  sleep 2
done

echo "redis-primary está disponível, iniciando Sentinel..."

# Resolve o IP de redis-primary
PRIMARY_IP=$(getent hosts redis-primary | awk '{print $1}' | head -n 1)
if [ -z "$PRIMARY_IP" ]; then
  echo "Erro: Não foi possível resolver o IP de redis-primary"
  exit 1
fi
echo "IP de redis-primary resolvido: $PRIMARY_IP"

# Cria um arquivo de configuração básico se não existir
if [ ! -f /etc/redis/sentinel.conf ]; then
  echo "port 26379" > /etc/redis/sentinel.conf
  chmod 644 /etc/redis/sentinel.conf
fi

# Inicia o Sentinel em background
redis-sentinel /etc/redis/sentinel.conf &
SENTINEL_PID=$!

# Aguarda o Sentinel estar completamente pronto
echo "Aguardando Sentinel iniciar..."
sleep 5

# Verifica se o Sentinel está ativo
if ! redis-cli -h localhost -p 26379 ping; then
  echo "Erro: Sentinel não está respondendo em localhost:26379"
  kill $SENTINEL_PID
  exit 1
fi

# Configura o monitoramento dinamicamente usando o IP
echo "Configurando Sentinel para monitorar mymaster com IP $PRIMARY_IP..."
redis-cli -h localhost -p 26379 SENTINEL MONITOR mymaster "$PRIMARY_IP" 6379 2 || {
  echo "Erro ao configurar MONITOR"
  kill $SENTINEL_PID
  exit 1
}
redis-cli -h localhost -p 26379 SENTINEL SET mymaster down-after-milliseconds 5000 || {
  echo "Erro ao configurar down-after-milliseconds"
  kill $SENTINEL_PID
  exit 1
}
redis-cli -h localhost -p 26379 SENTINEL SET mymaster failover-timeout 60000 || {
  echo "Erro ao configurar failover-timeout"
  kill $SENTINEL_PID
  exit 1
}
redis-cli -h localhost -p 26379 SENTINEL SET mymaster parallel-syncs 1 || {
  echo "Erro ao configurar parallel-syncs"
  kill $SENTINEL_PID
  exit 1
}

# 🔥 Correção importante para funcionar fora do Docker (Windows/macOS):
# Isso faz o Sentinel anunciar um IP que o host (seu app Go) consegue acessar.
redis-cli -h localhost -p 26379 SENTINEL SET mymaster announce-ip host.docker.internal || {
  echo "Erro ao configurar announce-ip"
  kill $SENTINEL_PID
  exit 1
}
redis-cli -h localhost -p 26379 SENTINEL SET mymaster announce-port 6379 || {
  echo "Erro ao configurar announce-port"
  kill $SENTINEL_PID
  exit 1
}

echo "Sentinel configurado com sucesso!"

# Mantém o container rodando
wait $SENTINEL_PID
