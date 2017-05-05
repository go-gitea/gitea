// Package git is a low level and highly extensible git client library for
// reading repositories from git servers.  It is written in Go from scratch,
// without any C dependencies.
//
// We have been following the open/close principle in its design to facilitate
// extensions.
//
// Small example extracting the commits from a repository:
//
//     func ExampleBasic_printCommits() {
//         r := git.NewMemoryRepository()
//         o := &git.CloneOptions{
//             URL: "https://github.com/src-d/go-git",
//         }
//         if err := r.Clone(o); err != nil {
//             panic(err)
//         }
//
//         iter, err := r.Commits()
//         if err != nil {
//             panic(err)
//         }
//         defer iter.Close()
//
//         for {
//             commit, err := iter.Next()
//             if err != nil {
//                 if err == io.EOF {
//                     break
//                 }
//                 panic(err)
//             }
//
//             fmt.Println(commit)
//         }
//    }
package git // import "gopkg.in/src-d/go-git.v4"
