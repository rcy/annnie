package config

import (
	"log"
	"sync"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	EvokeDB        string `env:"EVOKE_DB,required"`
	SQLiteDB       string `env:"SQLITE_DB,required"`
	IRCNick        string `env:"IRC_NICK,required"`
	IRCChannel     string `env:"IRC_CHANNEL,required"`
	IRCServer      string `env:"IRC_SERVER,required"`
	SASLLogin      string `env:"SASL_LOGIN,required"`
	SASLPassword   string `env:"SASL_PASSWORD,required"`
	OpenAIAPIKey   string `env:"OPENAI_API_KEY,required"`
	ImageFileBase  string `env:"IMAGE_FILE_BASE,required"`
	RootURL        string `env:"ROOT_URL,required"`

	Port             string `env:"PORT"`
	YouTubeAPIKey    string `env:"YOUTUBE_API_KEY"`
	TMDBAPIKey       string `env:"TMDB_API_KEY"`
	OpenWeatherAPIKey string `env:"OPENWEATHERMAP_API_KEY"`
	FootballDataAPIKey string `env:"FOOTBALL_DATA_API_KEY"`
	DeepSeekAPIKey   string `env:"DEEPSEEK_API_KEY"`
	OllamaAPIKey     string `env:"OLLAMA_API_KEY"`
	RunPodAPIKey     string `env:"RUNPOD_API_KEY"`
	GoldAPIToken     string `env:"GOLD_API_TOKEN"`
	GitHubToken      string `env:"GITHUB_TOKEN"`
	LuaGitRepo       string `env:"LUA_GIT_REPO"`
	LuaGitRemote     string `env:"LUA_GIT_REMOTE"`
	AnonymizeLinks   string `env:"ANONYMIZE_LINKS"`
	AnnieMCPEndpoints string `env:"ANNIE_MCP_ENDPOINTS"`
	IQAIRAPIKey      string `env:"IQAIR_API_KEY"`
}

var (
	cfg  Config
	once sync.Once
	err  error
)

func parse() {
	once.Do(func() { err = env.Parse(&cfg) })
}

func Get() Config {
	parse()
	return cfg
}

func Load() Config {
	parse()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	return cfg
}