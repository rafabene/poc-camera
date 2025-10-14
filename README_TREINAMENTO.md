# Como Melhorar e Personalizar a Detecção de Objetos

## 🚀 Status Atual do Projeto

Este projeto já está utilizando o **YOLOv11n Object365** com **365 classes**, que é **4.5x mais avançado** que o COCO tradicional (80 classes).

### 📊 Modelo Atual:
- **Modelo**: YOLOv11n Object365
- **Classes**: 365 (vs 80 do COCO)
- **Arquivo**: `models/yolo11n_object365.onnx`
- **Idioma**: Português brasileiro
- **Tamanho**: 10.7MB

### 🏷️ Classes Detectadas:
O modelo atual detecta objetos específicos como:
- **Calçados específicos**: tênis, outros sapatos, salto alto, botas, chinelos
- **Eletrônicos**: celular, laptop, tablet, fone de ouvido, câmera
- **Móveis detalhados**: cadeira, sofá, mesa de centro, criado-mudo
- **Instrumentos**: violão, piano, violino, saxofone, tambor
- **Comidas específicas**: abacaxi, manga, kiwi, durian, morango
- **Veículos**: SUV, carro esportivo, caminhonete, van

## 1. Modelos Ainda Mais Avançados

### Object365 vs Alternativas Modernas:

| Modelo | Classes | Vantagens |
|--------|---------|-----------|
| **COCO** | 80 | Básico, antigo |
| **Object365** ✅ | 365 | **Atual - muito bom** |
| **LVIS** | 1.203 | Mais classes, mas mais pesado |
| **OpenImages** | 600+ | Google, muito abrangente |

### Como Usar LVIS (1.203 classes):

```python
# Instalar dependências
pip install ultralytics huggingface_hub

# Baixar modelo LVIS (se disponível)
from huggingface_hub import hf_hub_download
model_path = hf_hub_download(repo_id="facebook/detectron2-lvis", filename="model.onnx")
```

## 2. Melhorar o Modelo Atual

### Ajustar Configurações no `main.go`:

```go
const (
    confidenceThreshold = 0.3  // 0.3-0.7: mais objetos detectados
    nmsThreshold        = 0.4  // 0.2-0.5: controla duplicatas
    minObjectSize       = 30   // 30-100: objetos menores
)
```

### Para Detectar Mais Objetos:
- **Diminuir** `confidenceThreshold` para `0.3`
- **Aumentar** `nmsThreshold` para `0.5`
- **Diminuir** `minObjectSize` para `30`

### Para Detecção Mais Precisa:
- **Aumentar** `confidenceThreshold` para `0.7`
- **Diminuir** `nmsThreshold` para `0.2`
- **Aumentar** `minObjectSize` para `100`

## 3. Treinar Modelo Personalizado

### Quando é Necessário:
- Objetos muito específicos (ex: peças industriais)
- Marcas específicas de produtos
- Objetos regionais/culturais únicos

### Processo com YOLOv11:

```python
from ultralytics import YOLO

# 1. Carregar modelo base Object365
model = YOLO('yolo11n.pt')

# 2. Treinar com dados customizados
results = model.train(
    data='meu_dataset.yaml',
    epochs=100,
    imgsz=640,
    batch=16,
    name='modelo_customizado'
)

# 3. Exportar para ONNX
model.export(format='onnx')
```

### Estrutura do Dataset:
```
meu_dataset/
├── images/
│   ├── train/     # 80% das imagens
│   └── val/       # 20% das imagens
├── labels/
│   ├── train/     # Arquivos .txt com anotações
│   └── val/       # Arquivos .txt com anotações
└── dataset.yaml   # Configuração
```

### Arquivo `dataset.yaml`:
```yaml
train: ./images/train
val: ./images/val
nc: 3  # número de novas classes
names: ['produto_a', 'produto_b', 'produto_c']
```

## 4. Modelos Especializados por Domínio

### Disponíveis no Hugging Face:

```bash
# Detecção em construção
pip install ultralytics
python -c "from ultralytics import YOLO; model = YOLO('yihong1120/Construction-Hazard-Detection-YOLO11')"

# Detecção de pragas agrícolas
python -c "from ultralytics import YOLO; model = YOLO('underdogquality/yolo11s-pest-detection')"

# Detecção de peixes
python -c "from ultralytics import YOLO; model = YOLO('akridge/yolo11-fish-detector-grayscale')"
```

## 5. Ferramentas para Anotação

### Recomendadas:
1. **LabelImg** (Gratuito): https://github.com/tzutalin/labelImg
2. **CVAT** (Web): https://cvat.ai
3. **Roboflow** (Pipeline completo): https://roboflow.com
4. **Label Studio**: https://labelstud.io

### Processo de Anotação:
1. Importe 100-500 imagens
2. Desenhe caixas ao redor dos objetos
3. Exporte no formato YOLO (.txt)
4. Organize conforme estrutura acima

## 6. Melhorias de Performance

### Pré-processamento de Imagem:
```go
// Melhorar contraste antes da detecção
processedImg := gocv.NewMat()
gocv.ConvertScaleAbs(img, &processedImg, 1.2, 30)
```

### Filtros Pós-detecção:
```go
// Filtrar por área
area := (box.Max.X - box.Min.X) * (box.Max.Y - box.Min.Y)
if area < 2500 { // menos que 50x50 pixels
    continue
}

// Filtrar por proporção
aspectRatio := float64(box.Max.X - box.Min.X) / float64(box.Max.Y - box.Min.Y)
if aspectRatio > 5.0 || aspectRatio < 0.2 { // muito alongado
    continue
}
```

## 7. Datasets Públicos para Treinamento

### Grandes Datasets:
- **LVIS**: 1.203 categorias - https://www.lvisdataset.org
- **Open Images V7**: 600+ categorias - https://storage.googleapis.com/openimages/web/index.html
- **Objects365**: 365 categorias - https://www.objects365.org
- **COCO**: 80 categorias - https://cocodataset.org

### Especializados:
- **WIDER Face**: Detecção de faces
- **CityScapes**: Condução autônoma
- **Pascal VOC**: Detecção geral
- **ImageNet**: Classificação de imagens

## 8. Próximos Passos Recomendados

### Para Melhorar Sem Treinar:
1. ✅ **Já implementado**: Object365 com 365 classes
2. 🔄 **Ajustar thresholds** conforme necessidade
3. 🔄 **Testar modelos LVIS** se precisar de mais classes

### Para Casos Específicos:
1. 📊 **Coletar dados** do seu domínio específico
2. 🏷️ **Anotar com LabelImg/Roboflow**
3. 🤖 **Treinar YOLOv11** com transfer learning
4. 📦 **Exportar para ONNX** e integrar

### Para Performance Máxima:
1. 🔧 **Implementar pré-processamento** de imagem
2. 🎯 **Adicionar filtros** pós-detecção
3. ⚡ **Usar GPU** se disponível
4. 📈 **Monitorar métricas** de precisão

## 9. Recursos e Comunidade

### Documentação Oficial:
- **Ultralytics YOLOv11**: https://docs.ultralytics.com
- **OpenCV Go**: https://gocv.io
- **Hugging Face Models**: https://huggingface.co/models?search=yolo11

### Comunidades:
- **GitHub Ultralytics**: Issues e discussões
- **Reddit r/MachineLearning**: Discussões técnicas
- **Discord/Slack**: Comunidades de CV

O modelo atual **Object365 já oferece excelente capacidade** para a maioria dos casos de uso. Considere treinamento personalizado apenas se precisar de objetos muito específicos não cobertos pelas 365 classes atuais.