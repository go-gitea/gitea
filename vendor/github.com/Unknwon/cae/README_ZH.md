压缩与打包扩展
=============

[![Go Walker](http://gowalker.org/api/v1/badge)](http://gowalker.org/github.com/Unknwon/cae)

包 cae 实现了 PHP 风格的压缩与打包扩展。

但本包依据 Go 语言的风格进行了一些修改。

引用：[PHP:Compression and Archive Extensions](http://www.php.net/manual/en/refs.compression.php)

编码规范：基于 [Go 编码规范](https://github.com/Unknwon/go-code-convention)

### 实现

包 `zip`([Go Walker](http://gowalker.org/github.com/Unknwon/cae/zip)) 和 `tz`([Go Walker](http://gowalker.org/github.com/Unknwon/cae/tz)) 都允许你轻易的读取或写入 ZIP/TAR.GZ 压缩档案和其内部文件。

- 特性：
	- 将任意位置的文件或目录加入档案，没有一对一的操作限制。
	- 只解压部分文件，而非一次性解压全部。 
	- 将数据以流的形式直接写入 `io.Writer` 而不需经过文件系统的存储。

### 测试用例与覆盖率

所有子包均采用 [GoConvey](http://goconvey.co/) 来书写测试用例，覆盖率均超过 80%。

## 授权许可

本项目采用 Apache v2 开源授权许可证，完整的授权说明已放置在 [LICENSE](LICENSE) 文件中。