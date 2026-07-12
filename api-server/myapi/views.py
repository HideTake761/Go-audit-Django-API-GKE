from rest_framework import viewsets
from .models import Item
from .serializers import ItemSerializer
from django_filters.rest_framework import DjangoFilterBackend

class ItemViewSet(viewsets.ModelViewSet):
    queryset = Item.objects.all() # Itemテーブルのレコードをすべて取り出す
    serializer_class = ItemSerializer
    # シリアライザーとしてItemSerializerを使用するよう指定
    # シリアライザー:
    # モデルのデータをJSON形式に変換したり、JSONデータをモデルインスタンスに
    # 変換したりするもの
    filterset_fields = ['product']
    # Enables REST API endpoints to GET, UPDATE, and DELETE items by product name
    filter_backends = [DjangoFilterBackend]
    # Set the filter backend to enable query parameter filtering
    # Django REST Frameworkでfilter_fieldsを使う場合、DjangoFilterBackendが
    # フィルタリングをサポートするために設定されている必要がある。
    # settings.pyにも設定必要
