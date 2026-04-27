"""豆包模型客户端"""
from ai_novel_studio.models.base import BaseModelClient
from ai_novel_studio.models.config import ModelConfig


class DoubaoClient(BaseModelClient):
    """豆包（Doubao）模型客户端"""

    def __init__(self, config: ModelConfig):
        super().__init__(config)

    async def generate(self, prompt: str, system_prompt: str = "", **kwargs) -> str:
        headers = {
            "Authorization": f"Bearer {self.config.api_key}",
            "Content-Type": "application/json",
        }
        payload = {
            "model": self.config.model_name,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": prompt},
            ],
            "temperature": kwargs.get("temperature", self.config.temperature),
            "max_tokens": kwargs.get("max_tokens", self.config.max_tokens),
        }
        response = await self.client.post(
            self.config.api_endpoint,
            headers=headers,
            json=payload,
        )
        response.raise_for_status()
        data = response.json()
        return data["choices"][0]["message"]["content"]
