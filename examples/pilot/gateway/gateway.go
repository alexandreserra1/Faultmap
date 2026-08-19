// Package gateway expõe um proxy reverso instrumentado que serve a um único
// propósito: fazer existir um segundo serviço no trace.
//
// Com uma aplicação só, o ranking do Faultmap não tem trabalho — há um único
// nome possível na lista de suspeitos. A pergunta que o produto foi feito para
// responder, "qual dos meus serviços é o culpado", só passa a existir quando um
// serviço chama outro e o contexto de trace atravessa a fronteira.
//
// Este proxy é deliberadamente burro. Ele não autentica, não limita taxa, não
// roteia por regra e não transforma payload — um gateway completo faria tudo
// isso e cada uma dessas funções seria um suspeito falso a mais quando algo
// desse errado no diagnóstico.
//
// O caso difícil que ele permite testar: quando o serviço de trás fica lento, o
// proxy também fica, porque está esperando a resposta. Os dois pioram juntos e
// com números parecidos. Acusar o proxy seria culpar a vítima.
package gateway

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// tempoLimite impede que uma requisição ao serviço de trás fique presa para
// sempre quando ele degrada, o que transformaria o proxy em acumulador de
// conexões e criaria um sintoma que não veio da falha injetada.
const tempoLimite = 30 * time.Second

// NewHandler cria o proxy reverso instrumentado para o destino informado.
func NewHandler(destino string) (http.Handler, error) {
	if strings.TrimSpace(destino) == "" {
		return nil, errors.New("destino do proxy é obrigatório")
	}
	alvo, err := url.Parse(strings.TrimSpace(destino))
	if err != nil {
		return nil, fmt.Errorf("destino do proxy inválido: %w", err)
	}
	if alvo.Scheme != "http" && alvo.Scheme != "https" {
		return nil, errors.New("destino do proxy deve ser uma URL HTTP ou HTTPS")
	}
	if alvo.Host == "" {
		return nil, errors.New("destino do proxy deve incluir host")
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(alvo)
			// SetXForwarded declara a origem sem apagar o que já veio, e o
			// cabeçalho de contexto de trace segue intacto: é ele que liga os
			// spans dos dois serviços no mesmo trace.
			request.SetXForwarded()
		},
		// O transporte instrumentado é o que injeta o contexto de trace na
		// chamada de saída. Sem ele o serviço de trás abriria um trace novo e o
		// grafo ficaria partido exatamente onde precisamos que ele exista.
		Transport: otelhttp.NewTransport(&http.Transport{
			ResponseHeaderTimeout: tempoLimite,
		}),
	}
	return proxy, nil
}

// WithDelayFile atrasa cada requisição pelo número de milissegundos escrito no
// arquivo indicado, quando ele existe.
//
// É o cenário em que o culpado é o próprio proxy, e serve de contraprova: se o
// Faultmap acusa o serviço de trás mesmo quando a lentidão nasce na frente, ele
// não está diagnosticando, está sempre apontando para o mesmo lado.
//
// A leitura acontece a cada requisição, de propósito. Ligar a falha por
// reinício partiria a janela do incidente em duas, com um intervalo morto no
// meio, e a comparação passaria a incluir o tempo em que nada respondia.
//
// Conteúdo inválido é tratado como ausência de atraso: um erro de escrita não
// pode derrubar o proxy no meio de um incidente.
func WithDelayFile(next http.Handler, caminho string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if atraso := delayFromFile(caminho); atraso > 0 {
			select {
			case <-time.After(atraso):
			case <-request.Context().Done():
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func delayFromFile(caminho string) time.Duration {
	conteúdo, err := os.ReadFile(caminho)
	if err != nil {
		return 0
	}
	milissegundos, err := strconv.Atoi(strings.TrimSpace(string(conteúdo)))
	if err != nil || milissegundos <= 0 {
		return 0
	}
	return time.Duration(milissegundos) * time.Millisecond
}
