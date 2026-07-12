from django.urls import path, include
from rest_framework.routers import DefaultRouter
from .views import ItemViewSet

router = DefaultRouter()
# DefaultRouter(): Creates URLs related to CRUD automatically
router.register(r'items', ItemViewSet)
# router.register():ビューセット(この場合はItemViewSet)を特定のURLパスに紐づける。
# r'items'のitemsは任意に設定でき、実際にURLパスとして使用される文字列
# 例:items/ (商品の一覧取得・作成用)、items/<int:pk>/ (特定の商品取得・更新・削除用)

urlpatterns = [  
    # path(アクセスするアドレス, 呼び出す処理)
    path('', include(router.urls)),
    # myapiアプリケーション内のURLを定義する際、Django REST Frameworkのルーターが
    # 自動生成したURLを、このアプリケーションのルートパス('')に含める
    # include():他のURLconf(URL設定ファイル)をインポートするために使われる
    # ここでは、ルーターが自動生成したURL設定のリストをこの場所に組み込んでいる
]
