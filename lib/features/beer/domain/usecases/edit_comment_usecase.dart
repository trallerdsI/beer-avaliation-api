import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecase/usecase.dart';
import '../entities/comment.dart';

class EditCommentParams {
  final int commentId;
  final int userId;
  final String newText;
  const EditCommentParams({required this.commentId, required this.userId, required this.newText});
}

class EditCommentUseCase implements UseCase<EditCommentParams, Comment> {
  EditCommentUseCase();

  @override
  Future<Either<Failure, Comment>> call(EditCommentParams params) async {
    final cleaned = params.newText.trim();
    if (cleaned.isEmpty) return Left(const Failure.unknown(message: 'Comentário vazio'));

    // No persistent comments repo yet — return a mock updated comment assuming ownership validated elsewhere.
    final updated = Comment(
      id: params.commentId,
      beerId: 0,
      userId: params.userId,
      text: cleaned,
      likesCount: 0,
    );

    return Right(updated);
  }
}
