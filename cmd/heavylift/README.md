# `heavylift` an archive command

`heavylift` moves large files from source to destination if the source file system is too full.

> [!IMPORTANT]
> Only support unix system, and depends on `rsync` command.

Under the hood, `heavylift` generate a list of files to be transferred, then use rsync to move them (deleting source file after copying). Lastly, the command looks at the file list again. For each entry it recursively checks if the folder is empty and if so, remove the folder.

If rsync returns non-zero exit code, we will print rsync output then perform the folder check anyway in case of partial success.

## Usage

```shell
go build ./cmd/heavylift

./heavylift [flag] <src> <dst>
```

* `src` the source to check for filesystem usage and to move files from.
    * Symlink are excluded and will never be followed.
* `dst` the destination to move files to.
    * New file path will be calculated as `dst + relative_path(src, original_path)`. For example: if `src` is `/home/me`, file is at `/home/me/large/file.foo` and `dst` is `/archive`, then the file will be moved to `/archive/large/file.foo`.
    * We don't check if `dst` and `src` are both on the same file system.

`heavylift` will prioritize moving largest files first.

Flags:
* `--older-than`:  only consider files older than some duration, eg `--older-than 30d`. Default is `45d` to consider files last modified >= 45 days ago.
* `--upper`: only start moving if src filesystem is more than x% full, eg. `--upper 60`. Default is `90`: move files if src file system is 90% full.
    * **Note**: command will terminate if src filesystem is not full yet.
* `--lower`: move files until src filesystem is less than x% full, eg. `--lower 40`. Default is `50`: move files until src filesystem is less than 50% full.
    * **Note**: depends on the specific `src` path it's possible that we will move all the files and still unable to meet the lower threshold. In this case, the command will just terminate.

## Why Rsync

rsync has code to handle probably every possible IO error out there, so I'd rather reuse it before reimplement it.
