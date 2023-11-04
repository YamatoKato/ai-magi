# ai-magi
## 概要
APIが公開されているテキスト生成AIモデルを利用して3つのAIに同じ質問をして、その結果を集約するエヴァに出てきた「MAGIシステム」を実現したい。
今回、私の今後の使用頻度と大学生の金銭的都合から、「GPT-3.5 Turbo」と「Claude-Instant v1」の2体に議題を投げ、それぞれの意見のまとめ役と結論付けを「claude v2」にやってもらうことにした。（余裕が生まれ次第、AIモデルを増やす...）

## 使用技術
### Go
今回のベース。AI系のAPI(SDK)とやたら相性の良いPythonでも良かったが、
- CLIで個人利用したい
  - CLI制作といえばGo
  - 個人利用のみでPublicなサービスにしないから
- 複数APIを叩く時にGoroutinで並行処理させて処理速度上げたい
  - それぞれ生成AIのレスポンス自体に時間がかかる
- 日本でAIモデルのAPIをGoで実装してる記事ほぼ無く、参考になればいいなって。。

ということでGoを採用。
### AWS Bedrock
GPT4よりも優秀と海外で話題のAnthropic製「Claude v2」のAPIを使う唯一の手段。「Claude-Instant v1」のAPIもBedrock経由で使うことになる。

### OpenAI API
皆さんお馴染み、ChatGPTでよく使う「GPT-3.5 Turbo」のAPIを利用する手段。
(余談だが、APIの無料期間が3ヶ月あると思い込んでいたが、実際にはChatGPT登録から3ヶ月の勘違いで、課金する羽目になった（ ;  ; ）)

## 利用
`.env`にそれぞれのAPIキーとAWSのIAMプロファイル（Bedrock full Access）のユーザー名を設定すれば、利用できると思います！

## デモ


https://github.com/YamatoKato/ai-magi/assets/95395675/e3a0267c-ca92-4143-b4fc-8ae1ff4c2be8

