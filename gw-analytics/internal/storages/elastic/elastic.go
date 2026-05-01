package elastic

import "github.com/elastic/go-elasticsearch/v9"

type Storage struct {
	client *elasticsearch.TypedClient
	index  string
}
