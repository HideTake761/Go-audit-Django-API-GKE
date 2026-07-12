from django.http import HttpResponse

def health_check(request):
    """
    ALBからのヘルスチェックに応答するためのシンプルなビュー
    常にHTTP 200 OKを返す
    """
    return HttpResponse("OK", status=200)