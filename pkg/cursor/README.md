# 游标分页

`Codec` 把列表位置编码为带签名的 URL 安全游标，并在解码时校验 scope、用户和签名。游标内容对调用方可读但不透明，调用方不能构造或修改位置。

```go
type postPosition struct {
    TimeMillis int64
    ID         int64
}

postsCursor := cursor.NewCodec[postPosition](signingKey, "posts:v1:"+env.Env())
token, err := postsCursor.Encode(postPosition{TimeMillis: last.TimeMillis, ID: last.ID}, userID)
position, err := postsCursor.Decode(req.Cursor, userID)
```

- key 由使用方在构造时注入，必须非空且不可预测；模板不提供该配置字段，不做游标分页的项目直接裁剪本包。
- scope 区分列表、环境和格式版本，改变排序或位置结构时更新版本段，旧游标随之失效。
- 游标与 `userID` 绑定；不按用户隔离的列表统一传 0。解码失败（编码、签名、scope 或用户不匹配）返回 `ErrInvalidCursor`，由 Service 转换为参数错误。
- 位置类型只包含定位所需的字段，例如排序时间与主键，不含业务数据。
- 本包只负责编解码和签名，不负责查询。Repo 使用相同的排序字段和方向构造 keyset 条件，并多取一条判断是否还有下一页。 [公共分页](../../conventions.md#公共分页)
