// Package identifier gera identificadores curtos, ordenáveis por tempo e
// derivados sem coordenação.
//
// O formato é o do ULID — 48 bits de instante em milissegundos seguidos de 80
// bits, escritos em 26 caracteres base32 de Crockford — com uma diferença que
// importa: os 80 bits finais não são aleatórios, são um resumo do conteúdo.
//
// A troca é deliberada. ULID e UUIDv7 tiram a unicidade de entropia, e este
// produto depende do oposto: a persistência é idempotente por
// `ON CONFLICT(id) DO NOTHING`, e o Faultmap promete que a mesma entrada produz
// a mesma saída. Com entropia aleatória, recoletar o mesmo catálogo gravaria
// tudo de novo como se fossem mudanças inéditas, e dois diagnósticos idênticos
// deixariam de ser comparáveis byte a byte.
//
// Substituir a entropia pelo resumo entrega as mesmas seis propriedades —
// singularidade, escalabilidade, desempenho, classificabilidade, tamanho
// compacto e ausência de coordenação — e mantém a promessa. O que se perde é a
// imprevisibilidade: quem conhece o conteúdo consegue recalcular o ID. Isso não
// é problema aqui, porque nenhum identificador do Faultmap é segredo nem serve
// de credencial; se algum dia servir, este pacote é o lugar errado para gerá-lo.
package identifier

import (
	"crypto/sha256"
	"strings"
	"time"
)

// crockford é o alfabeto base32 de Crockford, sem I, L, O e U. A exclusão
// evita que 1 e I, ou 0 e O, sejam confundidos por quem lê um identificador em
// um relatório, e impede que palavras se formem por acaso.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Length é o comprimento fixo, em caracteres, de todo identificador gerado.
const Length = 26

// New devolve o identificador do conteúdo observado no instante informado.
//
// É uma função pura: sem contador, sem sequência no banco, sem serviço externo.
// Um produto que se distribui como binário único não pode depender de
// coordenação para criar um registro, e o custo por chamada é um sha256 sobre
// poucas dezenas de bytes — nenhuma ida a disco ou rede no caminho da criação.
//
// A resolução é o milissegundo. Dois conteúdos distintos no mesmo milissegundo
// continuam distintos, porque o desempate vem do resumo; o mesmo conteúdo no
// mesmo milissegundo devolve sempre o mesmo identificador, que é o que sustenta
// a idempotência da persistência.
func New(instant time.Time, parts ...string) string {
	milliseconds := instant.UTC().UnixMilli()
	if milliseconds < 0 {
		// Instante anterior à época, ou zero. Ancorar em zero mantém o
		// identificador válido e ordenável em vez de produzir lixo.
		milliseconds = 0
	}

	// O separador impede que ["ab","c"] e ["a","bc"] produzam o mesmo resumo:
	// sem ele, a concatenação apagaria a fronteira entre as partes.
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))

	var raw [16]byte
	for index := 0; index < 6; index++ {
		raw[5-index] = byte(milliseconds >> (8 * index))
	}
	copy(raw[6:], digest[:10])
	return encode(raw)
}

// encode escreve os 128 bits em 26 caracteres base32, do mais significativo
// para o menos, para que a ordem dos bytes vire ordem dos caracteres.
func encode(raw [16]byte) string {
	var out [Length]byte
	for position := Length - 1; position >= 0; position-- {
		bit := (Length - 1 - position) * 5
		out[position] = crockford[extract(raw, bit)]
	}
	return string(out[:])
}

// extract lê os 5 bits que começam em offset contando do bit menos
// significativo do bloco de 128.
//
// Os 130 bits que 26 caracteres comportam são dois a mais que os 128
// disponíveis; a leitura acima do topo devolve zero, o que mantém o primeiro
// caractere estável e a ordenação intacta.
func extract(raw [16]byte, offset int) byte {
	var value byte
	for bit := 0; bit < 5; bit++ {
		position := offset + bit
		if position >= 128 {
			continue
		}
		byteIndex := 15 - position/8
		if raw[byteIndex]&(1<<(position%8)) != 0 {
			value |= 1 << bit
		}
	}
	return value
}
