# 目录属性表

本表是目录服务对外暴露的属性白名单。只有下表中列出的属性名可以作为过滤条件的左值。
属性名为不区分大小写匹配（目录服务侧行为），本仓库在判定时同样忽略大小写。

| 属性名 | 类型 | 说明 |
|---|---|---|
| `uid` | string | 登录工号 |
| `mail` | string | 邮箱地址 |
| `cn` | string | 显示名 |
| `sn` | string | 姓 |
| `givenName` | string | 名 |
| `displayName` | string | 展示用全名 |
| `departmentNumber` | string | 部门编号 |
| `ou` | string | 组织单元 |
| `title` | string | 职位 |
| `employeeNumber` | string | 员工编号 |
| `telephoneNumber` | string | 电话号码 |
| `mobile` | string | 手机号 |
| `manager` | string | 直属主管的 DN |
| `memberOf` | string | 所属组 |
| `objectClass` | string | 对象类型 |
| `employeeType` | string | 用工类型 |
| `l` | string | 城市 |
| `st` | string | 省/州 |
| `postalCode` | string | 邮政编码 |
| `status` | string | 账号状态 |

## 说明

- 表中每一行第一个反引号包裹的标识符即属性名，其余列均为描述性文本。
- 属性名中不含 `*`、空格、括号等字符；任何带通配符或空白的写法都不是本表中的属性名。
- 目录服务不提供 `objectClass*` 之类的属性名前缀通配能力；需要按类型筛选时请显式给出完整属性名。
