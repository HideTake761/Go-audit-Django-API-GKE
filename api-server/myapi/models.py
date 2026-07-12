from django.db import models
from django.core.validators import MinValueValidator
# Djangoのデータベース機能（モデル）を使うために、modelsモジュールをインポート

class Item(models.Model):
# Itemという名前の新しいモデル(DBのテーブルに対応)を作成、models.Modelを継承
    product = models.CharField(max_length=20)
    # productというフィールド(データベースの列に対応)を定義。文字列(CharField)で最大長は20文字
    price = models.IntegerField(validators=[MinValueValidator(0)])
    # priceというフィールドを定義。IntegerField():DBに整数を保存するためのフィールド
    # MinValueValidator(0):値が0以上であることを検証するためのバリデーター
    
    def __str__(self):
    # Itemモデルのインスタンスを表示すると、上で定義したproductフィールド(商品名)を返す
        return self.product
