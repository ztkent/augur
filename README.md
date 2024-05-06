# <img width="40" alt="logo_augur" src="https://github.com/Ztkent/augur/assets/7357311/b2a433f6-c611-4246-8c32-08517f9f07e7"> Augur

An assistant to rapidly align new LLM projects.  
Follows the best practices provided by [OpenAI](https://platform.openai.com/docs/guides/prompt-engineering/six-strategies-for-getting-better-results) 

## How Does It Work?
When a system prompt is requested:
- Evaluates the user's input and identifies the key themes and topics.
- Generates each section of the system prompt.
- Each section is reviewed to ensure it aligns with the desired structure and content.
- Presents the combined output. Provides the option to download the prompt in Markdown format.
- Options to regenerate sections, or the entire prompt.


## Infrastructure
- Frontend: HTML, TailwindCSS, HTMX
- Backend: Go
- Services: [OpenAI](https://platform.openai.com/docs/overview) (GPT-3.5/4), [Anyscale](https://www.anyscale.com/endpoints) (Open-source Models)

  
    
