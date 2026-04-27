"""
账号矩阵管理接口
CRUD 操作：创建、查询、更新、删除账号
"""
import logging
import uuid
from datetime import datetime
from typing import Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from sqlalchemy import select

from ai_novel_studio.storage.mysql import Account, AsyncSessionLocal

logger = logging.getLogger(__name__)

router = APIRouter()


class AccountCreate(BaseModel):
    platform: str
    agent_type: str
    username: str
    display_name: Optional[str] = None
    daily_quota: int = 3


class AccountUpdate(BaseModel):
    status: Optional[str] = None
    daily_quota: Optional[int] = None
    display_name: Optional[str] = None


def _account_to_dict(account: Account) -> dict:
    return {
        "id": account.id,
        "platform": account.platform,
        "agent_type": account.agent_type,
        "username": account.username,
        "display_name": account.display_name,
        "status": account.status,
        "daily_quota": account.daily_quota,
        "total_published": account.total_published,
        "created_at": account.created_at.isoformat() if account.created_at else None,
        "updated_at": account.updated_at.isoformat() if account.updated_at else None,
    }


@router.get("/")
async def list_accounts():
    """返回所有账号列表"""
    async with AsyncSessionLocal() as session:
        result = await session.execute(select(Account))
        accounts = result.scalars().all()
    return {"accounts": [_account_to_dict(a) for a in accounts], "total": len(accounts)}


@router.post("/", status_code=201)
async def create_account(data: AccountCreate):
    """创建新账号"""
    async with AsyncSessionLocal() as session:
        account = Account(
            id=str(uuid.uuid4()),
            platform=data.platform,
            agent_type=data.agent_type,
            username=data.username,
            display_name=data.display_name,
            status="active",
            daily_quota=data.daily_quota,
            total_published=0,
            created_at=datetime.utcnow(),
            updated_at=datetime.utcnow(),
        )
        session.add(account)
        await session.commit()
        await session.refresh(account)
        logger.info("账号创建成功: id=%s platform=%s", account.id, account.platform)
        return _account_to_dict(account)


@router.get("/{account_id}")
async def get_account(account_id: str):
    """获取单个账号详情"""
    async with AsyncSessionLocal() as session:
        account = await session.get(Account, account_id)
    if account is None:
        raise HTTPException(status_code=404, detail=f"账号不存在: account_id={account_id}")
    return _account_to_dict(account)


@router.patch("/{account_id}")
async def update_account(account_id: str, data: AccountUpdate):
    """更新账号信息（status / daily_quota / display_name）"""
    async with AsyncSessionLocal() as session:
        account = await session.get(Account, account_id)
        if account is None:
            raise HTTPException(status_code=404, detail=f"账号不存在: account_id={account_id}")

        if data.status is not None:
            account.status = data.status
        if data.daily_quota is not None:
            account.daily_quota = data.daily_quota
        if data.display_name is not None:
            account.display_name = data.display_name
        account.updated_at = datetime.utcnow()

        await session.commit()
        await session.refresh(account)
        logger.info("账号更新成功: id=%s", account_id)
        return _account_to_dict(account)


@router.delete("/{account_id}", status_code=204)
async def delete_account(account_id: str):
    """删除账号"""
    async with AsyncSessionLocal() as session:
        account = await session.get(Account, account_id)
        if account is None:
            raise HTTPException(status_code=404, detail=f"账号不存在: account_id={account_id}")
        await session.delete(account)
        await session.commit()
        logger.info("账号删除成功: id=%s", account_id)
